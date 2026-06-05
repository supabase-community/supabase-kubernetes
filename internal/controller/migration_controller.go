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
	"crypto/sha256"
	"fmt"
	"maps"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

const (
	migrationComponent = "migration"
	migrationDBUser    = "supabase_admin"
	kindSingleDatabase = "SingleDatabase"
)

// MigrationReconciler reconciles a Migration object
type MigrationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

// ResolvedMigrationDatabase holds resolved connection params for a migration.
type ResolvedMigrationDatabase struct {
	Host       string
	Port       int32
	DBName     string
	User       string
	SecretName string
	SecretKey  string
}

func (r *MigrationReconciler) resolveMigrationImage(migration *supabasev1alpha1.Migration) (string, error) {
	if migration.Spec.Image != "" {
		return migration.Spec.Image, nil
	}
	return ResolveComponentImage(migration.Spec.Version, "migration")
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
		r.setCondition(migration, metav1.ConditionFalse, "DatabaseResolutionFailed", err.Error())
		_ = r.updateStatus(ctx, migration)
		return ctrl.Result{}, err
	}
	if !dbReady {
		r.setCondition(migration, metav1.ConditionFalse, "DatabaseNotReady", "Waiting for database to be ready")
		if err := r.updateStatus(ctx, migration); err != nil {
			logger.Error(err, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	batchHash := calculateBatchHash(migration.Spec.Migrations)

	// If already applied with the same hash, nothing to do
	if migration.Status.AppliedHash == batchHash {
		r.setCondition(migration, metav1.ConditionTrue, "AllMigrationsApplied", "Migration batch already applied")
		if err := r.updateStatus(ctx, migration); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure ConfigMap exists
	configMapName := r.configMapName(migration.Name)
	if err := r.ensureConfigMap(ctx, migration, configMapName, batchHash); err != nil {
		logger.Error(err, "Failed to ensure ConfigMap for migration", "configmap", configMapName)
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "ConfigMapFailed", "ConfigMapCreationFailed", "Failed to ensure ConfigMap: %s", err.Error())
		r.setCondition(migration, metav1.ConditionFalse, "ConfigMapFailed", fmt.Sprintf("Failed to create ConfigMap: %s", err.Error()))
		_ = r.updateStatus(ctx, migration)
		return ctrl.Result{}, err
	}
	r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "ConfigMapCreated", "ConfigMapCreated", "ConfigMap %s ensured", configMapName)

	// Resolve migration image
	image, err := r.resolveMigrationImage(migration)
	if err != nil {
		logger.Error(err, "Failed to resolve migration image")
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "ImageResolutionFailed", "ImageResolutionFailed", "Failed to resolve migration image: %s", err.Error())
		r.setCondition(migration, metav1.ConditionFalse, "ImageResolutionFailed", err.Error())
		_ = r.updateStatus(ctx, migration)
		return ctrl.Result{}, err
	}

	// Check if job exists
	jobName := r.jobName(migration.Name)
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
		job = r.buildJob(migration, db, image, batchHash)
		if err := controllerutil.SetControllerReference(migration, job, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on job: %w", err)
		}
		if err := r.Create(ctx, job); err != nil {
			logger.Error(err, "Failed to create migration job", "job", jobName)
			r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "JobCreationFailed", "JobCreationFailed", "Failed to create migration job: %s", err.Error())
			r.setCondition(migration, metav1.ConditionFalse, "JobCreationFailed", fmt.Sprintf("Failed to create job: %s", err.Error()))
			_ = r.updateStatus(ctx, migration)
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "JobCreated", "JobCreated", "Migration job %s created", jobName)

		r.setCondition(migration, metav1.ConditionFalse, "Migrating", "Running migration batch")
		_ = r.updateStatus(ctx, migration)
		// Stop processing, wait for job to complete (requeue via Owns)
		return ctrl.Result{}, nil
	}

	// Job exists, check its status
	if job.Status.Succeeded > 0 {
		logger.Info("Migration batch completed successfully", "job", jobName, "hash", batchHash)
		migration.Status.AppliedHash = batchHash
		now := metav1.Now()
		migration.Status.AppliedAt = &now
		r.setCondition(migration, metav1.ConditionTrue, "AllMigrationsApplied", "All migrations applied successfully")
		r.Recorder.Eventf(migration, nil, corev1.EventTypeNormal, "MigrationsApplied", "MigrationsApplied", "Migration batch applied successfully (hash: %s)", batchHash)
		if err := r.updateStatus(ctx, migration); err != nil {
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
		r.setCondition(migration, metav1.ConditionFalse, "MigrationFailed", "Migration batch failed")
		r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "MigrationFailed", "MigrationFailed", "Migration batch failed (job: %s)", jobName)
		if err := r.updateStatus(ctx, migration); err != nil {
			logger.Error(err, "Failed to update status after failure")
			return ctrl.Result{}, err
		}
		// Do not clean up on failure so the Job logs are available for debugging
		return ctrl.Result{}, fmt.Errorf("migration job %s failed", jobName)
	}

	// Job is still running
	r.setCondition(migration, metav1.ConditionFalse, "Migrating", "Running migration batch")
	_ = r.updateStatus(ctx, migration)
	return ctrl.Result{}, nil
}

// updateStatus re-fetches the resource and applies the current status with retry on conflict.
func (r *MigrationReconciler) updateStatus(ctx context.Context, migration *supabasev1alpha1.Migration) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &supabasev1alpha1.Migration{}
		if err := r.Get(ctx, types.NamespacedName{Name: migration.Name, Namespace: migration.Namespace}, latest); err != nil {
			return err
		}
		latest.Status = migration.Status
		return r.Status().Update(ctx, latest)
	})
}

func (r *MigrationReconciler) cleanupResources(ctx context.Context, migration *supabasev1alpha1.Migration) {
	logger := log.FromContext(ctx)
	propagation := metav1.DeletePropagationBackground

	// Delete Job
	jobName := r.jobName(migration.Name)
	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: migration.Namespace}, job); err == nil {
		if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete migration job", "job", jobName)
		} else {
			logger.Info("Deleted migration job", "job", jobName)
		}
	}

	// Delete ConfigMap
	cmName := r.configMapName(migration.Name)
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: migration.Namespace}, cm); err == nil {
		if err := r.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete migration ConfigMap", "configmap", cmName)
		} else {
			logger.Info("Deleted migration ConfigMap", "configmap", cmName)
		}
	}
}

func (r *MigrationReconciler) resolveDatabaseRef(ctx context.Context, migration *supabasev1alpha1.Migration) (*ResolvedMigrationDatabase, bool, error) {
	ref := migration.Spec.DatabaseRef

	switch ref.Kind {
	case kindSingleDatabase:
		singleDB := &supabasev1alpha1.SingleDatabase{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: migration.Namespace}, singleDB); err != nil {
			if apierrors.IsNotFound(err) {
				r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "DatabaseNotFound", "DatabaseNotFound", "SingleDatabase %q not found, waiting", ref.Name)
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("getting SingleDatabase %q: %w", ref.Name, err)
		}
		if !meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady) {
			r.Recorder.Eventf(migration, nil, corev1.EventTypeWarning, "DatabaseNotReady", "DatabaseNotReady", "SingleDatabase %q is not ready, waiting", ref.Name)
			return nil, false, nil
		}
		return &ResolvedMigrationDatabase{
			Host:       fmt.Sprintf("%s.%s.svc.cluster.local", singleDB.Status.ServiceName, migration.Namespace),
			Port:       5432,
			DBName:     "postgres",
			User:       migrationDBUser,
			SecretName: singleDB.Status.SecretName,
			SecretKey:  "password",
		}, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported database kind %q", ref.Kind)
	}
}

func (r *MigrationReconciler) configMapName(migrationName string) string {
	return fmt.Sprintf("%s-sql", migrationName)
}

func (r *MigrationReconciler) jobName(migrationName string) string {
	return fmt.Sprintf("%s-apply", migrationName)
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

	batchSQL := buildBatchSQL(migration.Spec.Migrations, batchHash)

	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: migration.Namespace,
		},
		Data: map[string]string{
			"batch.sql": batchSQL,
		},
	}

	if err := controllerutil.SetControllerReference(migration, cm, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on configmap: %w", err)
	}

	return r.Create(ctx, cm)
}

func buildBatchSQL(entries []supabasev1alpha1.MigrationEntry, batchHash string) string {
	var b strings.Builder
	for i, entry := range entries {
		b.WriteString(fmt.Sprintf("-- migration %d: %s\n", i, entry.Name))
		b.WriteString(entry.SQL)
		b.WriteString("\n\n")
	}
	b.WriteString(fmt.Sprintf("INSERT INTO _migrations (hash) VALUES ('%s');\n", batchHash))
	return b.String()
}

func calculateBatchHash(entries []supabasev1alpha1.MigrationEntry) string {
	h := sha256.New()
	for _, entry := range entries {
		// Delimiter ensures concatenation is unambiguous
		h.Write([]byte(entry.SQL))
		h.Write([]byte("\x00MIGRATION\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (r *MigrationReconciler) buildJob(migration *supabasev1alpha1.Migration, db *ResolvedMigrationDatabase, image, batchHash string) *batchv1.Job {
	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(86400)
	configMapName := r.configMapName(migration.Name)

	script := assets.MigrationApplyScript

	env := make([]corev1.EnvVar, 0, 8+len(migration.Spec.Env))
	env = append(env,
		helper.EnvVarFromSecret("PGPASSWORD", db.SecretName, db.SecretKey),
		helper.EnvVarFromSecret("POSTGRES_PASSWORD", db.SecretName, db.SecretKey),
		helper.EnvVar("PGHOST", db.Host),
		helper.EnvVar("PGPORT", fmt.Sprintf("%d", db.Port)),
		helper.EnvVar("PGUSER", db.User),
		helper.EnvVar("POSTGRES_USER", db.User),
		helper.EnvVar("PGDATABASE", db.DBName),
		helper.EnvVar("MIGRATION_HASH", batchHash),
	)
	env = append(env, migration.Spec.Env...)

	container := corev1.Container{
		Name:            migrationComponent,
		Image:           image,
		ImagePullPolicy: migration.Spec.ImagePullPolicy,
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{script},
		Env:             env,
		Resources:       migration.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "migration-sql",
				MountPath: "/migrations",
				ReadOnly:  true,
			},
		},
	}

	if migration.Spec.ContainerSecurityContext != nil {
		container.SecurityContext = migration.Spec.ContainerSecurityContext
	}

	podLabels := maps.Clone(migration.Spec.PodLabels)
	if podLabels == nil {
		podLabels = map[string]string{}
	}

	podAnnotations := maps.Clone(migration.Spec.PodAnnotations)
	if podAnnotations == nil {
		podAnnotations = map[string]string{}
	}

	podSpec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyNever,
		Containers:                    []corev1.Container{container},
		NodeSelector:                  migration.Spec.NodeSelector,
		Affinity:                      migration.Spec.Affinity,
		Tolerations:                   migration.Spec.Tolerations,
		PriorityClassName:             migration.Spec.PriorityClassName,
		TerminationGracePeriodSeconds: migration.Spec.TerminationGracePeriodSeconds,
		Volumes: []corev1.Volume{
			{
				Name: "migration-sql",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			},
		},
	}

	if migration.Spec.PodSecurityContext != nil {
		podSpec.SecurityContext = migration.Spec.PodSecurityContext
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.jobName(migration.Name),
			Namespace: migration.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func (r *MigrationReconciler) setCondition(
	migration *supabasev1alpha1.Migration,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&migration.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             status,
		ObservedGeneration: migration.Generation,
		Reason:             reason,
		Message:            message,
	})
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
