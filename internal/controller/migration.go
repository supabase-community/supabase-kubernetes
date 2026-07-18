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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	migrationpkg "github.com/supabase-community/supabase-kubernetes/internal/migration"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

// MigrationReconciler reconciles a Migration object.
type MigrationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
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
	logger := log.FromContext(ctx).WithValues(
		"name", req.Name,
		"namespace", req.Namespace,
	)
	logger.Info("Starting Migration reconciliation")

	migration := &supabasev1alpha1.Migration{}
	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Migration resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Migration")
		return ctrl.Result{}, err
	}

	db, ready, err := database.ResolveRef(ctx, r.Client, migration.Spec.DatabaseRef, migration.Namespace)
	if err != nil {
		logger.Error(err, "Failed to resolve database reference")
		reconciler.SetNotReady(migration, "DatabaseResolutionFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after database resolution failure")
		}
		return ctrl.Result{}, err
	}
	if !ready {
		logger.Info("Database reference is not ready", "databaseRef", migration.Spec.DatabaseRef.Name)
		reconciler.SetNotReady(migration, "DatabaseNotReady", "Referenced database is not ready")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while waiting for database")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if err := r.ensureConfigMap(ctx, migration); err != nil {
		logger.Error(err, "Failed to ensure ConfigMap")
		reconciler.SetNotReady(migration, "ConfigMapFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after ConfigMap failure")
		}
		return ctrl.Result{}, err
	}

	if migration.Status.AppliedHash != "" {
		logger.Info("Migration already applied", "appliedHash", migration.Status.AppliedHash)
		return ctrl.Result{}, nil
	}

	if err := r.ensureJob(ctx, migration, db); err != nil {
		logger.Error(err, "Failed to ensure Job")
		reconciler.SetNotReady(migration, "JobFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after Job failure")
		}
		return ctrl.Result{}, err
	}

	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: migrationpkg.MigrationJobName(migration), Namespace: migration.Namespace}, job); err != nil {
		logger.Error(err, "Failed to get Job")
		reconciler.SetNotReady(migration, "JobGetFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after Job get failure")
		}
		return ctrl.Result{}, err
	}

	if job.Status.Failed > 0 {
		logger.Info("Migration Job failed", "failed", job.Status.Failed)
		reconciler.SetNotReady(migration, "JobFailed", "Migration Job failed")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after Job failed")
		}
		return ctrl.Result{}, fmt.Errorf("migration Job %s failed", job.Name)
	}

	if job.Status.Succeeded == 0 {
		logger.Info("Waiting for Job to complete")
		reconciler.SetNotReady(migration, "JobInProgress", "Waiting for migration Job to complete")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, migration); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while Job in progress")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if err := r.markReady(ctx, migration); err != nil {
		logger.Error(err, "Failed to update Migration status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *MigrationReconciler) ensureConfigMap(ctx context.Context, migration *supabasev1alpha1.Migration) error {
	cm, err := migrationpkg.MigrationConfigMap(migration)
	if err != nil {
		return fmt.Errorf("building configmap: %w", err)
	}
	if cm == nil {
		return reconciler.DeleteConfigMapIfExists(ctx, r.Client, migrationpkg.MigrationConfigMapName(migration), migration.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", cm.GetName(),
		"namespace", cm.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, cm, migration, reconciler.MutateConfigMap())
	if err != nil {
		return fmt.Errorf("ensuring configmap: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created ConfigMap")
	case reconciler.ResultUpdated:
		logger.Info("Updated ConfigMap")
	default:
		logger.V(1).Info("ConfigMap unchanged")
	}

	return nil
}

func (r *MigrationReconciler) ensureJob(ctx context.Context, migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) error {
	job, err := migrationpkg.MigrationJob(migration, db)
	if err != nil {
		return fmt.Errorf("building job: %w", err)
	}
	if job == nil {
		return reconciler.DeleteJobIfExists(ctx, r.Client, migrationpkg.MigrationJobName(migration), migration.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", job.GetName(),
		"namespace", job.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, job, migration, reconciler.MutateJob())
	if err != nil {
		return fmt.Errorf("ensuring job: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Job")
	case reconciler.ResultUpdated:
		logger.Info("Updated Job")
	default:
		logger.V(1).Info("Job unchanged")
	}

	return nil
}

func (r *MigrationReconciler) markReady(ctx context.Context, migration *supabasev1alpha1.Migration) error {
	now := metav1.Now()

	migration.Status.AppliedHash = migrationpkg.MigrationHash(migration)
	migration.Status.AppliedAt = &now

	reconciler.SetReady(migration, "ReconcileSucceeded", "Migration applied successfully")
	return reconciler.UpdateStatus(ctx, r.Client, migration)
}
