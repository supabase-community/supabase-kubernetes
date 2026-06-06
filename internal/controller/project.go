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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"maps"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	"github.com/supabase-community/supabase-kubernetes/internal/images"
	projectpkg "github.com/supabase-community/supabase-kubernetes/internal/project"
)

const (
	DefaultJWTExpiry = 24 * time.Hour * 365 * 10

	ConditionTypeSecretsReady      = "SecretsReady"
	ConditionTypeReady             = "Ready"
	ConditionTypeDatabaseReady     = "DatabaseReady"
	ConditionTypeMigrationReady    = "MigrationReady"
	ConditionTypeJWTSettingsReady  = "JWTSettingsReady"
	ConditionTypePasswordSyncReady = "PasswordSyncReady"

	DefaultMigrationNameSuffix = "-migration"
)

type secretDefinition struct {
	suffix    string
	generator func() (map[string][]byte, error)
}

// ProjectReconciler reconciles a Project object.
type ProjectReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

// projectReconciler returns a project package reconciler backed by the same
// client and scheme used by the controller reconciler.
func (r *ProjectReconciler) projectReconciler() *projectpkg.Reconciler {
	return &projectpkg.Reconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases/status,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=rests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=rests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=auths,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=auths/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=meta,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=meta/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=realtimes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=realtimes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles the reconciliation loop for Project resources.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	project := &supabasev1alpha1.Project{}
	if err := r.Get(ctx, req.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Project resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Project")
		return ctrl.Result{}, err
	}

	if err := r.ensureAllSecrets(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure secrets")
		r.setCondition(project, ConditionTypeSecretsReady, metav1.ConditionFalse, "SecretGenerationFailed", err.Error())
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "SecretsNotReady", "Generated secrets are not ready")
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after secret failure")
		}
		return ctrl.Result{}, err
	}

	db, err := r.resolveDatabaseRef(ctx, project)
	if err != nil {
		logger.Error(err, "Failed to resolve database reference")
		r.setCondition(project, ConditionTypeDatabaseReady, metav1.ConditionFalse, "DatabaseResolutionFailed", err.Error())
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "DatabaseNotReady", "Database reference could not be resolved")
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after database resolution failure")
		}
		return ctrl.Result{}, err
	}

	project.Status.ResolvedDatabase = db

	migrationResult, err := r.ensureMigration(ctx, project)
	if err != nil {
		logger.Error(err, "Failed to ensure migration")
		r.setCondition(project, ConditionTypeMigrationReady, metav1.ConditionFalse, "MigrationFailed", err.Error())
		r.setCondition(project, ConditionTypeDatabaseReady, metav1.ConditionFalse, "MigrationNotReady", "Database migration failed")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "MigrationNotReady", "Built-in migration failed or not ready")
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after migration failure")
		}
		return ctrl.Result{}, err
	}
	if migrationResult.RequeueAfter > 0 {
		r.setCondition(project, ConditionTypeMigrationReady, metav1.ConditionFalse, "MigrationInProgress", "Built-in migration is running")
		r.setCondition(project, ConditionTypeDatabaseReady, metav1.ConditionFalse, "MigrationInProgress", "Waiting for built-in migration to complete")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "MigrationNotReady", "Waiting for built-in migration to complete")
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while migration in progress")
		}
		return migrationResult, nil
	}

	r.setCondition(project, ConditionTypeMigrationReady, metav1.ConditionTrue, "MigrationApplied", "Built-in migration applied successfully")
	r.setCondition(project, ConditionTypeDatabaseReady, metav1.ConditionTrue, "DatabaseResolved", "Database reference resolved and migration applied")
	r.setCondition(project, ConditionTypeSecretsReady, metav1.ConditionTrue, "AllSecretsReady", "All generated secrets are present and complete")

	jwtResult, err := r.ensureJWTSettings(ctx, project)
	if err != nil {
		logger.Error(err, "Failed to ensure JWT settings")
		r.setCondition(project, ConditionTypeJWTSettingsReady, metav1.ConditionFalse, "JWTSettingsFailed", err.Error())
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "JWTSettingsNotReady", err.Error())
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after JWT settings failure")
		}
		return ctrl.Result{}, err
	}
	if jwtResult.RequeueAfter > 0 {
		r.setCondition(project, ConditionTypeJWTSettingsReady, metav1.ConditionFalse, "JWTSettingsInProgress", "Waiting for JWT settings sync to complete")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "JWTSettingsNotReady", "Waiting for JWT settings sync to complete")
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while JWT settings in progress")
		}
		return jwtResult, nil
	}

	r.setCondition(project, ConditionTypeJWTSettingsReady, metav1.ConditionTrue, "JWTSettingsApplied", "JWT settings applied successfully")

	passwordSyncResult, err := r.ensurePasswordSync(ctx, project)
	if err != nil {
		logger.Error(err, "Failed to ensure password sync")
		r.setCondition(project, ConditionTypePasswordSyncReady, metav1.ConditionFalse, "PasswordSyncFailed", err.Error())
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "PasswordSyncNotReady", err.Error())
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after password sync failure")
		}
		return ctrl.Result{}, err
	}
	if passwordSyncResult.RequeueAfter > 0 {
		r.setCondition(project, ConditionTypePasswordSyncReady, metav1.ConditionFalse, "PasswordSyncInProgress", "Waiting for password sync to complete")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "PasswordSyncNotReady", "Waiting for password sync to complete")
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while password sync in progress")
		}
		return passwordSyncResult, nil
	}

	r.setCondition(project, ConditionTypePasswordSyncReady, metav1.ConditionTrue, "PasswordSyncApplied", "Password sync applied successfully")

	if err := r.projectReconciler().EnsureRest(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Rest")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "RestNotReady", err.Error())
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update Project status")
		}
		return ctrl.Result{}, err
	}

	if err := r.projectReconciler().EnsureMeta(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Meta")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "MetaNotReady", err.Error())
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update Project status")
		}
		return ctrl.Result{}, err
	}

	if err := r.projectReconciler().EnsureRealtime(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Realtime")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "RealtimeNotReady", err.Error())
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update Project status")
		}
		return ctrl.Result{}, err
	}

	if err := r.projectReconciler().EnsureAuth(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Auth")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "AuthNotReady", err.Error())
		if statusErr := r.updateProjectStatus(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update Project status")
		}
		return ctrl.Result{}, err
	}

	r.setCondition(project, ConditionTypeReady, metav1.ConditionTrue, "ReconcileSucceeded", "All resources reconciled successfully")
	if err := r.updateProjectStatus(ctx, project); err != nil {
		logger.Error(err, "Failed to update Project status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

// secretDefinitions returns the list of secrets that must exist for a Project.
//
//nolint:unparam
func (r *ProjectReconciler) secretDefinitions(project *supabasev1alpha1.Project) []secretDefinition {
	return []secretDefinition{
		{
			suffix: "jwt",
			generator: func() (map[string][]byte, error) {
				return GenerateJWTSecretData(time.Now(), DefaultJWTExpiry)
			},
		},
		{suffix: "keys", generator: func() (map[string][]byte, error) { return GenerateKeysSecretData() }},
	}
}

// ensureAllSecrets iterates over all secret definitions and ensures each one exists with all required keys.
func (r *ProjectReconciler) ensureAllSecrets(ctx context.Context, project *supabasev1alpha1.Project) error {
	for _, def := range r.secretDefinitions(project) {
		secretName := fmt.Sprintf("%s-%s", project.Name, def.suffix)
		if err := r.ensureSecret(ctx, project, secretName, def.generator); err != nil {
			return fmt.Errorf("ensuring secret %q: %w", secretName, err)
		}
	}
	return nil
}

// ensureSecret ensures a Kubernetes Secret exists with all required keys.
func (r *ProjectReconciler) ensureSecret(
	ctx context.Context,
	owner *supabasev1alpha1.Project,
	name string,
	generator func() (map[string][]byte, error),
) error {
	logger := log.FromContext(ctx).WithValues("secret", name)

	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: owner.Namespace}, existing)

	if apierrors.IsNotFound(err) {
		logger.Info("Creating generated secret")

		data, genErr := generator()
		if genErr != nil {
			return fmt.Errorf("generating data for new secret: %w", genErr)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: owner.Namespace,
			},
			Data: data,
		}

		if err := controllerutil.SetControllerReference(owner, secret, r.Scheme); err != nil {
			return fmt.Errorf("setting owner reference: %w", err)
		}

		if err := r.Create(ctx, secret); err != nil {
			return fmt.Errorf("creating secret: %w", err)
		}

		logger.Info("Created generated secret")
		return nil
	}

	if err != nil {
		return fmt.Errorf("getting existing secret: %w", err)
	}

	data, genErr := generator()
	if genErr != nil {
		return fmt.Errorf("generating data to check missing keys: %w", genErr)
	}

	missingKeys := make(map[string][]byte)
	for key, val := range data {
		if _, found := existing.Data[key]; !found {
			missingKeys[key] = val
		}
	}

	if len(missingKeys) == 0 {
		logger.V(1).Info("Secret is complete, no missing keys")
		return nil
	}

	logger.Info("Patching missing keys into secret", "missingKeys", keysOf(missingKeys))

	if existing.Data == nil {
		existing.Data = make(map[string][]byte)
	}
	maps.Copy(existing.Data, missingKeys)

	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating secret with missing keys: %w", err)
	}

	logger.Info("Patched missing keys into secret")
	return nil
}

// setCondition sets a status condition on the Project.
func (r *ProjectReconciler) setCondition(
	project *supabasev1alpha1.Project,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: project.Generation,
		Reason:             reason,
		Message:            message,
	})
}

// updateProjectStatus re-fetches the resource and applies the current status with retry on conflict.
func (r *ProjectReconciler) updateProjectStatus(ctx context.Context, project *supabasev1alpha1.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &supabasev1alpha1.Project{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, latest); err != nil {
			return err
		}
		latest.Status = project.Status
		return r.Status().Update(ctx, latest)
	})
}

func (r *ProjectReconciler) resolveDatabaseRef(ctx context.Context, project *supabasev1alpha1.Project) (*supabasev1alpha1.ResolvedDatabase, error) {
	db, ready, err := database.ResolveRef(ctx, r.Client, project.Spec.DatabaseRef, project.Namespace)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, fmt.Errorf("database reference %q is not ready", project.Spec.DatabaseRef.Name)
	}
	return db, nil
}

// keysOf returns the keys of a map as a string slice (for logging).
func keysOf(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (r *ProjectReconciler) migrationName(project *supabasev1alpha1.Project, index int) string {
	return fmt.Sprintf("%s%s-%d", project.Name, DefaultMigrationNameSuffix, index)
}

func (r *ProjectReconciler) buildMigration(project *supabasev1alpha1.Project, index int, files []string) (*supabasev1alpha1.Migration, error) {
	entries, err := projectpkg.LoadMigrationEntries(files)
	if err != nil {
		return nil, fmt.Errorf("loading default migrations: %w", err)
	}
	return &supabasev1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.migrationName(project, index),
			Namespace: project.Namespace,
		},
		Spec: supabasev1alpha1.MigrationSpec{
			Version:     project.Spec.Version,
			DatabaseRef: project.Spec.DatabaseRef,
			Migrations:  entries,
		},
	}, nil
}

func (r *ProjectReconciler) ensureMigration(ctx context.Context, project *supabasev1alpha1.Project) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	appliedHashes := []string{}

	for i, batch := range projectpkg.DefaultMigrations {
		migrationName := r.migrationName(project, i)
		migrationLogger := logger.WithValues("migration", migrationName)

		migration := &supabasev1alpha1.Migration{}
		err := r.Get(ctx, types.NamespacedName{Name: migrationName, Namespace: project.Namespace}, migration)

		if err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("getting migration %s: %w", migrationName, err)
			}

			migrationLogger.Info("Creating built-in migration")
			migration, err = r.buildMigration(project, i, batch)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("building migration %s: %w", migrationName, err)
			}
			if err := controllerutil.SetControllerReference(project, migration, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting owner reference on migration %s: %w", migrationName, err)
			}
			if err := r.Create(ctx, migration); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating migration %s: %w", migrationName, err)
			}
			migrationLogger.Info("Created built-in migration")
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}

		cond := meta.FindStatusCondition(migration.Status.Conditions, ConditionTypeReady)
		if cond != nil && cond.Status == metav1.ConditionTrue {
			appliedHashes = append(appliedHashes, migration.Status.AppliedHash)
			continue
		}

		if cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "MigrationFailed" {
			return ctrl.Result{}, fmt.Errorf("migration %s failed: %s", migrationName, cond.Message)
		}

		migrationLogger.Info("Waiting for migration to complete")
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	project.Status.AppliedMigrationHash = calculateCombinedHash(appliedHashes)
	logger.Info("All migrations applied", "hash", project.Status.AppliedMigrationHash)
	return ctrl.Result{}, nil
}

func calculateCombinedHash(hashes []string) string {
	h := sha256.New()
	for _, hash := range hashes {
		h.Write([]byte(hash))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (r *ProjectReconciler) jwtSyncJobName(project *supabasev1alpha1.Project) string {
	return project.Name + "-sync-jwt"
}

func (r *ProjectReconciler) calculateJWTSettingsHash(secretData []byte, expirySeconds int32) string {
	h := sha256.New()
	h.Write(secretData)
	h.Write([]byte(strconv.Itoa(int(expirySeconds))))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (r *ProjectReconciler) ensureJWTSettings(ctx context.Context, project *supabasev1alpha1.Project) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	jwtSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: project.Name + "-jwt", Namespace: project.Namespace}, jwtSecret); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("JWT secret not found, skipping JWT settings sync")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting JWT secret: %w", err)
	}

	expirySeconds := int32(3600)
	if project.Spec.JWTExpirySeconds != nil {
		expirySeconds = *project.Spec.JWTExpirySeconds
	}

	currentHash := r.calculateJWTSettingsHash(jwtSecret.Data["jwt-secret"], expirySeconds)

	if project.Annotations != nil && project.Annotations["core.supabase.io/last-applied-sync-jwt-hash"] == currentHash {
		return ctrl.Result{}, nil
	}

	jobName := r.jwtSyncJobName(project)
	job := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: project.Namespace}, job)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("getting JWT sync job: %w", err)
		}

		logger.Info("Creating JWT settings sync job", "job", jobName)
		job = r.buildJWTSettingsJob(project, expirySeconds)
		if err := controllerutil.SetControllerReference(project, job, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on JWT sync job: %w", err)
		}
		if err := r.Create(ctx, job); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating JWT sync job: %w", err)
		}
		logger.Info("Created JWT settings sync job", "job", jobName)
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if job.Status.Succeeded > 0 {
		if project.Annotations == nil {
			project.Annotations = map[string]string{}
		}
		project.Annotations["core.supabase.io/last-applied-sync-jwt-hash"] = currentHash
		if err := r.Update(ctx, project); err != nil {
			logger.Error(err, "Failed to update project annotation after JWT sync", "job", jobName)
			return ctrl.Result{}, fmt.Errorf("updating project annotation after JWT sync: %w", err)
		}

		propagation := metav1.DeletePropagationBackground
		if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete JWT sync job", "job", jobName)
		} else {
			logger.Info("Deleted JWT sync job", "job", jobName)
		}

		logger.Info("JWT settings sync completed", "job", jobName)
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		return ctrl.Result{}, fmt.Errorf("JWT sync job %s failed", jobName)
	}

	logger.Info("Waiting for JWT settings sync to complete", "job", jobName)
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

func (r *ProjectReconciler) buildJWTSettingsJob(project *supabasev1alpha1.Project, expirySeconds int32) *batchv1.Job {
	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(86400)

	image, _ := images.Resolve(project.Spec.Version, images.ComponentMigration)

	env := []corev1.EnvVar{
		helper.EnvVarFromSecret("PGPASSWORD", project.Status.ResolvedDatabase.PasswordRef.Name, project.Status.ResolvedDatabase.PasswordRef.Key),
		helper.EnvVar("PGHOST", project.Status.ResolvedDatabase.Host),
		helper.EnvVar("PGPORT", fmt.Sprintf("%d", project.Status.ResolvedDatabase.Port)),
		helper.EnvVar("PGUSER", "supabase_admin"),
		helper.EnvVar("PGDATABASE", project.Status.ResolvedDatabase.DBName),
		helper.EnvVarFromSecret("JWT_SECRET", project.Name+"-jwt", "jwt-secret"),
		helper.EnvVar("JWT_EXP", strconv.Itoa(int(expirySeconds))),
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.jwtSyncJobName(project),
			Namespace: project.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "sync-jwt",
							Image:   image,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{assets.ProjectSyncJWTScript},
							Env:     env,
						},
					},
				},
			},
		},
	}
}

func (r *ProjectReconciler) passwordSyncJobName(project *supabasev1alpha1.Project) string {
	return project.Name + "-sync-password"
}

func (r *ProjectReconciler) calculatePasswordHash(secretData []byte) string {
	h := sha256.New()
	h.Write(secretData)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (r *ProjectReconciler) ensurePasswordSync(ctx context.Context, project *supabasev1alpha1.Project) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if project.Status.ResolvedDatabase == nil {
		logger.Info("Resolved database not available, skipping password sync")
		return ctrl.Result{}, nil
	}

	passwordRef := project.Status.ResolvedDatabase.PasswordRef
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: passwordRef.Name, Namespace: project.Namespace}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Password secret not found, skipping password sync")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting password secret: %w", err)
	}

	password := string(secret.Data[passwordRef.Key])
	currentHash := r.calculatePasswordHash([]byte(password))

	if project.Annotations != nil && project.Annotations["core.supabase.io/last-applied-password-hash"] == currentHash {
		return ctrl.Result{}, nil
	}

	jobName := r.passwordSyncJobName(project)
	job := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: project.Namespace}, job)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("getting password sync job: %w", err)
		}

		logger.Info("Creating password sync job", "job", jobName)
		job = r.buildPasswordSyncJob(project, password)
		if err := controllerutil.SetControllerReference(project, job, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on password sync job: %w", err)
		}
		if err := r.Create(ctx, job); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating password sync job: %w", err)
		}
		logger.Info("Created password sync job", "job", jobName)
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if job.Status.Succeeded > 0 {
		if project.Annotations == nil {
			project.Annotations = map[string]string{}
		}
		project.Annotations["core.supabase.io/last-applied-password-hash"] = currentHash
		if err := r.Update(ctx, project); err != nil {
			logger.Error(err, "Failed to update project annotation after password sync", "job", jobName)
			return ctrl.Result{}, fmt.Errorf("updating project annotation after password sync: %w", err)
		}

		propagation := metav1.DeletePropagationBackground
		if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete password sync job", "job", jobName)
		} else {
			logger.Info("Deleted password sync job", "job", jobName)
		}

		logger.Info("Password sync completed", "job", jobName)
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		return ctrl.Result{}, fmt.Errorf("password sync job %s failed", jobName)
	}

	logger.Info("Waiting for password sync to complete", "job", jobName)
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

func (r *ProjectReconciler) buildPasswordSyncJob(project *supabasev1alpha1.Project, password string) *batchv1.Job {
	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(86400)

	image, _ := images.Resolve(project.Spec.Version, images.ComponentMigration)

	env := []corev1.EnvVar{
		helper.EnvVar("PGPASSWORD", password),
		helper.EnvVar("PGHOST", project.Status.ResolvedDatabase.Host),
		helper.EnvVar("PGPORT", fmt.Sprintf("%d", project.Status.ResolvedDatabase.Port)),
		helper.EnvVar("PGUSER", "supabase_admin"),
		helper.EnvVar("PGDATABASE", project.Status.ResolvedDatabase.DBName),
		helper.EnvVar("DB_ADMIN_USER", "supabase_admin"),
		helper.EnvVar("DB_SRV_NAME", fmt.Sprintf("%s.%s.svc.cluster.local", project.Status.ResolvedDatabase.Host, project.Namespace)),
		helper.EnvVar("DB_SRV_PORT", fmt.Sprintf("%d", project.Status.ResolvedDatabase.Port)),
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.passwordSyncJobName(project),
			Namespace: project.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "sync-password",
							Image:   image,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{assets.ProjectSyncPasswordScript},
							Env:     env,
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	singleDatabaseToProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		singleDB, ok := obj.(*supabasev1alpha1.SingleDatabase)
		if !ok {
			return nil
		}

		projectList := &supabasev1alpha1.ProjectList{}
		if err := r.List(ctx, projectList, client.InNamespace(singleDB.Namespace)); err != nil {
			return nil
		}

		var requests []reconcile.Request
		for i := range projectList.Items {
			if projectList.Items[i].Spec.DatabaseRef.Kind == "SingleDatabase" &&
				projectList.Items[i].Spec.DatabaseRef.Name == singleDB.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      projectList.Items[i].Name,
						Namespace: projectList.Items[i].Namespace,
					},
				})
			}
		}
		return requests
	})

	migrationToProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		migration, ok := obj.(*supabasev1alpha1.Migration)
		if !ok {
			return nil
		}
		for _, ref := range migration.OwnerReferences {
			if ref.Kind == "Project" {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      ref.Name,
						Namespace: migration.Namespace,
					},
				}}
			}
		}
		return nil
	})

	restToProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		rest, ok := obj.(*supabasev1alpha1.Rest)
		if !ok {
			return nil
		}
		projectList := &supabasev1alpha1.ProjectList{}
		if err := r.List(ctx, projectList, client.InNamespace(rest.Namespace)); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range projectList.Items {
			if projectList.Items[i].Spec.RestRef != nil && projectList.Items[i].Spec.RestRef.Name == rest.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      projectList.Items[i].Name,
						Namespace: projectList.Items[i].Namespace,
					},
				})
			}
		}
		return requests
	})

	metaToProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		m, ok := obj.(*supabasev1alpha1.Meta)
		if !ok {
			return nil
		}
		projectList := &supabasev1alpha1.ProjectList{}
		if err := r.List(ctx, projectList, client.InNamespace(m.Namespace)); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range projectList.Items {
			if projectList.Items[i].Spec.MetaRef != nil && projectList.Items[i].Spec.MetaRef.Name == m.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      projectList.Items[i].Name,
						Namespace: projectList.Items[i].Namespace,
					},
				})
			}
		}
		return requests
	})

	realtimeToProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		rt, ok := obj.(*supabasev1alpha1.Realtime)
		if !ok {
			return nil
		}
		projectList := &supabasev1alpha1.ProjectList{}
		if err := r.List(ctx, projectList, client.InNamespace(rt.Namespace)); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range projectList.Items {
			if projectList.Items[i].Spec.RealtimeRef != nil && projectList.Items[i].Spec.RealtimeRef.Name == rt.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      projectList.Items[i].Name,
						Namespace: projectList.Items[i].Namespace,
					},
				})
			}
		}
		return requests
	})

	authToProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		auth, ok := obj.(*supabasev1alpha1.Auth)
		if !ok {
			return nil
		}
		projectList := &supabasev1alpha1.ProjectList{}
		if err := r.List(ctx, projectList, client.InNamespace(auth.Namespace)); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range projectList.Items {
			if projectList.Items[i].Spec.AuthRef != nil && projectList.Items[i].Spec.AuthRef.Name == auth.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      projectList.Items[i].Name,
						Namespace: projectList.Items[i].Namespace,
					},
				})
			}
		}
		return requests
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&supabasev1alpha1.Project{}).
		Watches(&supabasev1alpha1.SingleDatabase{}, singleDatabaseToProject).
		Watches(&supabasev1alpha1.Migration{}, migrationToProject).
		Watches(&supabasev1alpha1.Rest{}, restToProject).
		Watches(&supabasev1alpha1.Meta{}, metaToProject).
		Watches(&supabasev1alpha1.Realtime{}, realtimeToProject).
		Watches(&supabasev1alpha1.Auth{}, authToProject).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("project").
		Complete(r)
}

// GenerateJWTSecretData generates all key material for the JWT secret (9 keys).
func GenerateJWTSecretData(now time.Time, jwtExpiry time.Duration) (map[string][]byte, error) {
	jwtSecretBytes := make([]byte, 30)
	if _, err := rand.Read(jwtSecretBytes); err != nil {
		return nil, fmt.Errorf("generating jwt-secret bytes: %w", err)
	}
	jwtSecret := base64.StdEncoding.EncodeToString(jwtSecretBytes)

	anonKey, err := helper.GenerateJWTToken(jwtSecret, "anon", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating anon-key: %w", err)
	}

	serviceKey, err := helper.GenerateJWTToken(jwtSecret, "service_role", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating service-key: %w", err)
	}

	ecKey, err := helper.GenerateECP256Keypair()
	if err != nil {
		return nil, fmt.Errorf("generating EC P-256 keypair: %w", err)
	}

	kid, err := helper.GenerateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generating kid: %w", err)
	}

	ecPrivateJWK := helper.ECPrivateKeyToJWK(ecKey, kid)
	ecPublicJWK := helper.ECPublicKeyToJWK(ecKey, kid)
	octJWK := helper.SymmetricKeyToJWK(jwtSecret)

	jwtKeys, err := helper.BuildJWTKeys(ecPrivateJWK, octJWK)
	if err != nil {
		return nil, fmt.Errorf("building jwt-keys: %w", err)
	}

	jwtJWKS, err := helper.BuildJWTJWKS(ecPublicJWK, octJWK)
	if err != nil {
		return nil, fmt.Errorf("building jwt-jwks: %w", err)
	}

	anonKeyAsym, err := helper.SignES256JWT(ecKey, kid, "anon", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating anon-key-asymmetric: %w", err)
	}

	serviceKeyAsym, err := helper.SignES256JWT(ecKey, kid, "service_role", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating service-key-asymmetric: %w", err)
	}

	publishableKey, err := helper.GenerateOpaqueKey("sb_publishable_")
	if err != nil {
		return nil, fmt.Errorf("generating publishable-key: %w", err)
	}

	secretKey, err := helper.GenerateOpaqueKey("sb_secret_")
	if err != nil {
		return nil, fmt.Errorf("generating secret-key: %w", err)
	}

	return map[string][]byte{
		"jwt-secret":             []byte(jwtSecret),
		"anon-key":               []byte(anonKey),
		"service-key":            []byte(serviceKey),
		"jwt-keys":               []byte(jwtKeys),
		"jwt-jwks":               []byte(jwtJWKS),
		"anon-key-asymmetric":    []byte(anonKeyAsym),
		"service-key-asymmetric": []byte(serviceKeyAsym),
		"publishable-key":        []byte(publishableKey),
		"secret-key":             []byte(secretKey),
	}, nil
}

// GenerateKeysSecretData generates the shared keys secret data.
func GenerateKeysSecretData() (map[string][]byte, error) {
	secretKeyBase, err := helper.GenerateRandomHex(64)
	if err != nil {
		return nil, fmt.Errorf("generating secret-key-base: %w", err)
	}

	cryptoKey, err := helper.GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating crypto-key: %w", err)
	}

	vaultEncKey, err := helper.GenerateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generating vault-enc-key: %w", err)
	}

	encoded, err := helper.GenerateSAMLPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generating saml-private-key: %w", err)
	}

	return map[string][]byte{
		"secret-key-base":  []byte(secretKeyBase),
		"crypto-key":       []byte(cryptoKey),
		"vault-enc-key":    []byte(vaultEncKey),
		"saml-private-key": []byte(encoded),
	}, nil
}
