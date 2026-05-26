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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	databaseMigrationComponent = "migration"
	migrationDBUser            = "supabase_admin"
	kindSingleDatabase         = "SingleDatabase"
	kindExternalDatabase       = "ExternalDatabase"
)

// DatabaseMigrationReconciler reconciles a DatabaseMigration object
type DatabaseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ResolvedMigrationDatabase holds resolved connection params for a migration.
type ResolvedMigrationDatabase struct {
	Host       string
	Port       int32
	DBName     string
	User       string
	SecretName string
	SecretKey  string
	Image      string
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=databasemigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=databasemigrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=databasemigrations/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=externaldatabases,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles the reconciliation loop for DatabaseMigration resources.
func (r *DatabaseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	migration := &platformv1alpha1.DatabaseMigration{}
	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DatabaseMigration")
		return ctrl.Result{}, err
	}

	db, err := r.resolveDatabaseRef(ctx, migration)
	if err != nil {
		logger.Error(err, "Failed to resolve database reference")
		r.setCondition(migration, metav1.ConditionFalse, "DatabaseResolutionFailed", err.Error())
		_ = r.updateStatus(ctx, migration)
		return ctrl.Result{}, err
	}

	// Ensure migrationStatuses slice is initialized for all entries
	r.ensureStatusSlice(migration)

	// Process migrations sequentially
	totalMigrations := len(migration.Spec.Migrations)
	appliedCount := 0

	for i, entry := range migration.Spec.Migrations {
		stepStatus := &migration.Status.MigrationStatuses[i]

		// Already applied, continue to next
		if stepStatus.Applied {
			appliedCount++
			continue
		}

		// Ensure ConfigMap exists for this migration step
		configMapName := r.configMapName(migration.Name, i)
		if err := r.ensureConfigMap(ctx, migration, entry, configMapName); err != nil {
			logger.Error(err, "Failed to ensure ConfigMap for migration", "configmap", configMapName)
			r.setCondition(migration, metav1.ConditionFalse, "ConfigMapFailed", fmt.Sprintf("Failed to create ConfigMap for migration %q: %s", entry.Name, err.Error()))
			_ = r.updateStatus(ctx, migration)
			return ctrl.Result{}, err
		}

		// Check if job exists for this migration step
		jobName := r.jobName(migration.Name, i)
		job := &batchv1.Job{}
		err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: migration.Namespace}, job)

		if err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "Failed to get migration job", "job", jobName)
				return ctrl.Result{}, err
			}

			// Job does not exist, create it
			logger.Info("Creating migration job", "job", jobName, "migration", entry.Name, "index", i)
			job = r.buildJob(migration, db, i)
			if err := controllerutil.SetControllerReference(migration, job, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting owner reference on job: %w", err)
			}
			if err := r.Create(ctx, job); err != nil {
				logger.Error(err, "Failed to create migration job", "job", jobName)
				r.setCondition(migration, metav1.ConditionFalse, "JobCreationFailed", fmt.Sprintf("Failed to create job for migration %q: %s", entry.Name, err.Error()))
				_ = r.updateStatus(ctx, migration)
				return ctrl.Result{}, err
			}

			stepStatus.JobName = jobName
			r.setCondition(migration, metav1.ConditionFalse, "Migrating", fmt.Sprintf("Running migration %d/%d: %s", i+1, totalMigrations, entry.Name))
			_ = r.updateStatus(ctx, migration)
			// Stop processing, wait for job to complete (requeue via Owns)
			return ctrl.Result{}, nil
		}

		// Job exists, check its status
		if job.Status.Succeeded > 0 {
			logger.Info("Migration step completed successfully", "job", jobName, "migration", entry.Name)
			stepStatus.Applied = true
			now := metav1.Now()
			stepStatus.AppliedAt = &now
			stepStatus.JobName = jobName
			appliedCount++
			// Continue to next migration
			continue
		}

		if job.Status.Failed > 0 {
			logger.Info("Migration step failed", "job", jobName, "migration", entry.Name)
			stepStatus.JobName = jobName
			r.setCondition(migration, metav1.ConditionFalse, "MigrationFailed", fmt.Sprintf("Migration %d/%d failed: %s", i+1, totalMigrations, entry.Name))
			_ = r.updateStatus(ctx, migration)
			// Stop processing, do not continue with subsequent migrations
			return ctrl.Result{}, fmt.Errorf("migration job %s failed for step %q", jobName, entry.Name)
		}

		// Job is still running
		stepStatus.JobName = jobName
		r.setCondition(migration, metav1.ConditionFalse, "Migrating", fmt.Sprintf("Running migration %d/%d: %s", i+1, totalMigrations, entry.Name))
		_ = r.updateStatus(ctx, migration)
		// Stop processing, wait for job to complete (requeue via Owns)
		return ctrl.Result{}, nil
	}

	// All migrations applied - clean up Jobs and ConfigMaps
	r.cleanupResources(ctx, migration)

	r.setCondition(migration, metav1.ConditionTrue, "AllMigrationsApplied", fmt.Sprintf("All %d migrations applied successfully", appliedCount))
	if err := r.updateStatus(ctx, migration); err != nil {
		logger.Error(err, "Failed to update status after all migrations applied")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateStatus re-fetches the resource and applies the current status with retry on conflict.
func (r *DatabaseMigrationReconciler) updateStatus(ctx context.Context, migration *platformv1alpha1.DatabaseMigration) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &platformv1alpha1.DatabaseMigration{}
		if err := r.Get(ctx, types.NamespacedName{Name: migration.Name, Namespace: migration.Namespace}, latest); err != nil {
			return err
		}
		latest.Status = migration.Status
		return r.Status().Update(ctx, latest)
	})
}

func (r *DatabaseMigrationReconciler) cleanupResources(ctx context.Context, migration *platformv1alpha1.DatabaseMigration) {
	logger := log.FromContext(ctx)
	propagation := metav1.DeletePropagationBackground

	for i := range migration.Spec.Migrations {
		// Delete Job
		jobName := r.jobName(migration.Name, i)
		job := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: migration.Namespace}, job); err == nil {
			if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "Failed to delete migration job", "job", jobName)
			} else {
				logger.Info("Deleted migration job", "job", jobName)
			}
		}

		// Delete ConfigMap
		cmName := r.configMapName(migration.Name, i)
		cm := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: migration.Namespace}, cm); err == nil {
			if err := r.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "Failed to delete migration ConfigMap", "configmap", cmName)
			} else {
				logger.Info("Deleted migration ConfigMap", "configmap", cmName)
			}
		}
	}

}

func (r *DatabaseMigrationReconciler) ensureStatusSlice(migration *platformv1alpha1.DatabaseMigration) {
	desired := len(migration.Spec.Migrations)
	current := len(migration.Status.MigrationStatuses)

	if current >= desired {
		return
	}

	// Extend the status slice for new entries
	for i := current; i < desired; i++ {
		migration.Status.MigrationStatuses = append(migration.Status.MigrationStatuses, platformv1alpha1.MigrationStepStatus{
			Name: migration.Spec.Migrations[i].Name,
		})
	}
}

func (r *DatabaseMigrationReconciler) resolveDatabaseRef(ctx context.Context, migration *platformv1alpha1.DatabaseMigration) (*ResolvedMigrationDatabase, error) {
	ref := migration.Spec.DatabaseRef

	switch ref.Kind {
	case kindSingleDatabase:
		singleDB := &platformv1alpha1.SingleDatabase{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: migration.Namespace}, singleDB); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("SingleDatabase %q not found", ref.Name)
			}
			return nil, fmt.Errorf("getting SingleDatabase %q: %w", ref.Name, err)
		}
		if !meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady) {
			return nil, fmt.Errorf("SingleDatabase %q is not ready", ref.Name)
		}
		return &ResolvedMigrationDatabase{
			Host:       fmt.Sprintf("%s.%s.svc.cluster.local", singleDB.Status.ServiceName, migration.Namespace),
			Port:       5432,
			DBName:     "postgres",
			User:       migrationDBUser,
			SecretName: singleDB.Status.SecretName,
			SecretKey:  "password",
			Image:      migration.Spec.Image,
		}, nil
	case kindExternalDatabase:
		extDB := &platformv1alpha1.ExternalDatabase{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: migration.Namespace}, extDB); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("ExternalDatabase %q not found", ref.Name)
			}
			return nil, fmt.Errorf("getting ExternalDatabase %q: %w", ref.Name, err)
		}
		return &ResolvedMigrationDatabase{
			Host:       extDB.Spec.Host,
			Port:       derefInt32(extDB.Spec.Port, 5432),
			DBName:     derefString(extDB.Spec.DBName, "postgres"),
			User:       migrationDBUser,
			SecretName: extDB.Spec.PasswordRef.Name,
			SecretKey:  extDB.Spec.PasswordRef.Key,
			Image:      migration.Spec.Image,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported database kind %q", ref.Kind)
	}
}

func (r *DatabaseMigrationReconciler) configMapName(migrationName string, index int) string {
	return fmt.Sprintf("%s-%d-sql", migrationName, index)
}

func (r *DatabaseMigrationReconciler) jobName(migrationName string, index int) string {
	return fmt.Sprintf("%s-%d", migrationName, index)
}

func (r *DatabaseMigrationReconciler) ensureConfigMap(ctx context.Context, migration *platformv1alpha1.DatabaseMigration, entry platformv1alpha1.MigrationEntry, name string) error {
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: migration.Namespace}, cm)
	if err == nil {
		// ConfigMap already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// Create ConfigMap with the SQL content
	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: migration.Namespace,
		},
		Data: map[string]string{
			"migration.sql": entry.SQL,
		},
	}

	if err := controllerutil.SetControllerReference(migration, cm, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on configmap: %w", err)
	}

	return r.Create(ctx, cm)
}

func (r *DatabaseMigrationReconciler) buildJob(migration *platformv1alpha1.DatabaseMigration, db *ResolvedMigrationDatabase, index int) *batchv1.Job {
	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(86400)
	entry := migration.Spec.Migrations[index]
	configMapName := r.configMapName(migration.Name, index)

	script := `set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Create migrations tracking table if not exists
psql -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());"

# Check if already applied (idempotency at DB level)
ALREADY_APPLIED=$(psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM _migrations WHERE name = '$MIGRATION_NAME';")

if [ "$ALREADY_APPLIED" = "1" ]; then
    echo "Migration $MIGRATION_NAME already applied, skipping"
    exit 0
fi

# Apply migration from mounted ConfigMap (no shell interpretation of SQL content)
psql -v ON_ERROR_STOP=1 -f /migrations/migration.sql

# Register migration as applied
psql -v ON_ERROR_STOP=1 -c "INSERT INTO _migrations (name) VALUES ('$MIGRATION_NAME');"

echo "Migration $MIGRATION_NAME applied successfully"
`

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.jobName(migration.Name, index),
			Namespace: migration.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    databaseMigrationComponent,
							Image:   db.Image,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{script},
							Env: []corev1.EnvVar{
								envVarFromSecret("PGPASSWORD", db.SecretName, db.SecretKey),
								envVarFromSecret("POSTGRES_PASSWORD", db.SecretName, db.SecretKey),
								envVar("PGHOST", db.Host),
								envVar("PGPORT", fmt.Sprintf("%d", db.Port)),
								envVar("PGUSER", db.User),
								envVar("POSTGRES_USER", db.User),
								envVar("PGDATABASE", db.DBName),
								envVar("MIGRATION_NAME", entry.Name),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "migration-sql",
									MountPath: "/migrations",
									ReadOnly:  true,
								},
							},
						},
					},
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
				},
			},
		},
	}
}

func (r *DatabaseMigrationReconciler) setCondition(
	migration *platformv1alpha1.DatabaseMigration,
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
func (r *DatabaseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.DatabaseMigration{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Named("databasemigration").
		Complete(r)
}
