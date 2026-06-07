/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	"github.com/supabase-community/supabase-kubernetes/internal/images"
	migpkg "github.com/supabase-community/supabase-kubernetes/internal/migration"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

// MigrationReconciler reconciles a Migration object
type MigrationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

func (r *MigrationReconciler) resolveMigrationImage(migration *supabasev1alpha1.Migration) string {
	if migration.Spec.Image != "" {
		return migration.Spec.Image
	}
	return images.Resolve(migration.Spec.Version, images.ComponentMigration)
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=migrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=migrations/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for Migration resources.
func (r *MigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	migration := &supabasev1alpha1.Migration{}
	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Migration")
		return ctrl.Result{}, err
	}

	r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "Reconciling", "Reconciling", "Starting reconciliation of Migration %s", migration.Name)

	db, dbReady, err := r.resolveDatabaseRef(ctx, migration)
	if err != nil {
		logger.Error(err, "Failed to resolve database reference")
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "DatabaseResolutionFailed", "DatabaseResolutionFailed", "Failed to resolve database reference: %s", err.Error())
		reconciler.SetNotReady(migration, "DatabaseResolutionFailed", err.Error())
		_ = reconciler.UpdateStatus(ctx, r.Client, migration)
		return ctrl.Result{}, err
	}
	if !dbReady {
		reconciler.SetNotReady(migration, "DatabaseNotReady", "Waiting for database to be ready")
		if err := reconciler.UpdateStatus(ctx, r.Client, migration); err != nil {
			logger.Error(err, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	batchHash := migpkg.CalculateBatchHash(migration.Spec.Migrations)

	// If already applied with the same hash, nothing to do
	if migration.Status.AppliedHash == batchHash {
		reconciler.SetReady(migration, "AllMigrationsApplied", "Migration batch already applied")
		if err := reconciler.UpdateStatus(ctx, r.Client, migration); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure ConfigMap exists
	configMapName := migpkg.ConfigMapName(migration.Name)
	if err := r.ensureConfigMap(ctx, migration, configMapName, batchHash); err != nil {
		logger.Error(err, "Failed to ensure ConfigMap for migration", "configmap", configMapName)
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "ConfigMapFailed", "ConfigMapCreationFailed", "Failed to ensure ConfigMap: %s", err.Error())
		reconciler.SetNotReady(migration, "ConfigMapFailed", fmt.Sprintf("Failed to create ConfigMap: %s", err.Error()))
		_ = reconciler.UpdateStatus(ctx, r.Client, migration)
		return ctrl.Result{}, err
	}
	r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "ConfigMapCreated", "ConfigMapCreated", "ConfigMap %s ensured", configMapName)

	// Resolve migration image
	image := r.resolveMigrationImage(migration)

	// Check if job exists
	jobName := migpkg.JobName(migration.Name)
	job := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: migration.Namespace}, job)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to get migration job", "job", jobName)
			r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "JobGetFailed", "JobGetFailed", "Failed to get migration job: %s", err.Error())
			return ctrl.Result{}, err
		}

		// Job does not exist, create it
		logger.Info("Creating migration job", "job", jobName, "hash", batchHash)
		job = migpkg.BuildJob(migration, db, image, batchHash)
		if err := controllerutil.SetControllerReference(migration, job, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on job: %w", err)
		}
		if err := r.Create(ctx, job); err != nil {
			logger.Error(err, "Failed to create migration job", "job", jobName)
			r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "JobCreationFailed", "JobCreationFailed", "Failed to create migration job: %s", err.Error())
			reconciler.SetNotReady(migration, "JobCreationFailed", fmt.Sprintf("Failed to create job: %s", err.Error()))
			_ = reconciler.UpdateStatus(ctx, r.Client, migration)
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "JobCreated", "JobCreated", "Migration job %s created", jobName)

		reconciler.SetNotReady(migration, "Migrating", "Running migration batch")
		_ = reconciler.UpdateStatus(ctx, r.Client, migration)
		// Stop processing, wait for job to complete (requeue via Owns)
		return ctrl.Result{}, nil
	}

	// Job exists, check its status
	if job.Status.Succeeded > 0 {
		logger.Info("Migration batch completed successfully", "job", jobName, "hash", batchHash)
		migration.Status.AppliedHash = batchHash
		now := metav1.Now()
		migration.Status.AppliedAt = &now
		reconciler.SetReady(migration, "AllMigrationsApplied", "All migrations applied successfully")
		r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "MigrationsApplied", "MigrationsApplied", "Migration batch applied successfully (hash: %s)", batchHash)
		if err := reconciler.UpdateStatus(ctx, r.Client, migration); err != nil {
			logger.Error(err, "Failed to update status after success")
			r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "StatusUpdateFailed", "StatusUpdateFailed", "Failed to update status: %s", err.Error())
			return ctrl.Result{}, err
		}
		// Clean up resources on success
		r.cleanupResources(ctx, migration)
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		logger.Info("Migration batch failed", "job", jobName, "hash", batchHash)
		reconciler.SetNotReady(migration, "MigrationFailed", "Migration batch failed")
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "MigrationFailed", "MigrationFailed", "Migration batch failed (job: %s)", jobName)
		if err := reconciler.UpdateStatus(ctx, r.Client, migration); err != nil {
			logger.Error(err, "Failed to update status after failure")
			return ctrl.Result{}, err
		}
		// Do not clean up on failure so the Job logs are available for debugging
		return ctrl.Result{}, fmt.Errorf("migration job %s failed", jobName)
	}

	// Job is still running
	reconciler.SetNotReady(migration, "Migrating", "Running migration batch")
	_ = reconciler.UpdateStatus(ctx, r.Client, migration)
	return ctrl.Result{}, nil
}

func (r *MigrationReconciler) cleanupResources(ctx context.Context, migration *supabasev1alpha1.Migration) {
	logger := log.FromContext(ctx)
	propagation := metav1.DeletePropagationBackground

	// Delete Job
	jobName := migpkg.JobName(migration.Name)
	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: migration.Namespace}, job); err == nil {
		if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete migration job", "job", jobName)
		} else {
			logger.Info("Deleted migration job", "job", jobName)
		}
	}

	// Delete ConfigMap
	cmName := migpkg.ConfigMapName(migration.Name)
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: migration.Namespace}, cm); err == nil {
		if err := r.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete migration ConfigMap", "configmap", cmName)
		} else {
			logger.Info("Deleted migration ConfigMap", "configmap", cmName)
		}
	}
}

func (r *MigrationReconciler) resolveDatabaseRef(ctx context.Context, migration *supabasev1alpha1.Migration) (*supabasev1alpha1.ResolvedDatabase, bool, error) {
	db, ready, err := database.ResolveRef(ctx, r.Client, migration.Spec.DatabaseRef, migration.Namespace)
	if err != nil {
		return nil, false, err
	}
	if !ready {
		ref := migration.Spec.DatabaseRef
		// Determine whether it is not-found or not-ready so we can emit the right event.
		singleDB := &supabasev1alpha1.SingleDatabase{}
		if getErr := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: migration.Namespace}, singleDB); getErr != nil {
			if apierrors.IsNotFound(getErr) {
				r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "DatabaseNotFound", "DatabaseNotFound", "SingleDatabase %q not found, waiting", ref.Name)
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("getting SingleDatabase %q: %w", ref.Name, getErr)
		}
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "DatabaseNotReady", "DatabaseNotReady", "SingleDatabase %q is not ready, waiting", ref.Name)
		return nil, false, nil
	}
	return db, true, nil
}

func (r *MigrationReconciler) ensureConfigMap(ctx context.Context, migration *supabasev1alpha1.Migration, name string, batchHash string) error {
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: migration.Namespace}, cm)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	cm = migpkg.BuildConfigMap(migration, name, batchHash)

	if err := controllerutil.SetControllerReference(migration, cm, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on configmap: %w", err)
	}

	return r.Create(ctx, cm)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&supabasev1alpha1.Migration{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Named("migration").
		Complete(r)
}
