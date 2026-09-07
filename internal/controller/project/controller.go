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

package project

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

// Reconciler reconciles a Project object.
type Reconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&supabasev1alpha1.Project{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Watches(
			&supabasev1alpha1.SingleDatabase{},
			handler.EnqueueRequestsFromMapFunc(r.mapSingleDatabaseToProjects),
		).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.mapSecretToProjects)).
		Named("project").
		Complete(r)
}

func (r *Reconciler) mapSingleDatabaseToProjects(ctx context.Context, obj client.Object) []reconcile.Request {
	singleDB, ok := obj.(*supabasev1alpha1.SingleDatabase)
	if !ok {
		return nil
	}

	projects := &supabasev1alpha1.ProjectList{}
	if err := r.List(ctx, projects, client.InNamespace(singleDB.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, proj := range projects.Items {
		if proj.Spec.DatabaseRef.Kind == "SingleDatabase" && proj.Spec.DatabaseRef.Name == singleDB.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      proj.Name,
					Namespace: proj.Namespace,
				},
			})
		}
	}
	return requests
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases/status,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/status,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for Project resources.
//
//nolint:gocyclo // keep the reconciliation steps explicit
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"name", req.Name,
		"namespace", req.Namespace,
	)
	logger.Info("Starting Project reconciliation")

	proj := &supabasev1alpha1.Project{}
	if err := r.Get(ctx, req.NamespacedName, proj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Project resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Project")
		return ctrl.Result{}, err
	}

	db, ready, err := database.ResolveRef(ctx, r.Client, proj.Spec.DatabaseRef, proj.Namespace)
	if err != nil {
		logger.Error(err, "Failed to resolve database reference")
		reconciler.SetNotReady(proj, "DatabaseResolutionFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after database resolution failure")
		}
		return ctrl.Result{}, err
	}
	if !ready {
		logger.Info("Database reference is not ready", "databaseRef", proj.Spec.DatabaseRef.Name)
		reconciler.SetNotReady(proj, "DatabaseNotReady", "Referenced database is not ready")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while waiting for database")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if err := r.ensureJWTSecret(ctx, proj); err != nil {
		logger.Error(err, "Failed to ensure JWT Secret")
		reconciler.SetNotReady(proj, "JWTSecretFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after JWT secret failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureKeysSecret(ctx, proj); err != nil {
		logger.Error(err, "Failed to ensure Keys Secret")
		reconciler.SetNotReady(proj, "KeysSecretFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after keys secret failure")
		}
		return ctrl.Result{}, err
	}

	jwtSecret, err := r.getJWTSecretValue(ctx, proj)
	if err != nil {
		logger.Error(err, "Failed to get JWT secret value")
		reconciler.SetNotReady(proj, "JWTSecretValueFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after JWT secret value failure")
		}
		return ctrl.Result{}, err
	}

	dbPassword, err := r.getDBPasswordValue(ctx, proj.Namespace, db)
	if err != nil {
		logger.Error(err, "Failed to get database password value")
		reconciler.SetNotReady(proj, "DBPasswordValueFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after database password value failure")
		}
		return ctrl.Result{}, err
	}

	if project.ComputeJWTSyncHash(project.NewContext(proj), db, dbPassword, jwtSecret) != proj.Status.JWTSyncHash {
		if err := r.ensureSyncJWTJob(ctx, proj, db); err != nil {
			logger.Error(err, "Failed to ensure SyncJWTJob")
			reconciler.SetNotReady(proj, "SyncJWTJobFailed", err.Error())
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status after sync JWT job failure")
			}
			return ctrl.Result{}, err
		}

		syncJWTJob := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.SyncJWTJobName(project.NewContext(proj)), Namespace: proj.Namespace}, syncJWTJob); err != nil {
			logger.Error(err, "Failed to get SyncJWTJob")
			return ctrl.Result{}, err
		}
		if syncJWTJob.Status.Failed > 0 {
			logger.Info("SyncJWTJob failed")
			reconciler.SetNotReady(proj, "SyncJWTJobFailed", "Sync JWT job failed")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status after sync JWT job failure")
			}
			return ctrl.Result{}, fmt.Errorf("sync-jwt job failed")
		}
		if syncJWTJob.Status.Succeeded == 0 {
			logger.Info("Waiting for SyncJWTJob to complete")
			reconciler.SetNotReady(proj, "SyncJWTJobInProgress", "Waiting for sync JWT job to complete")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for sync JWT job")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}

		proj.Status.JWTSyncHash = project.ComputeJWTSyncHash(project.NewContext(proj), db, dbPassword, jwtSecret)
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update JWTSyncHash status")
			return ctrl.Result{}, statusErr
		}
	}

	if project.ComputePasswordSyncHash(project.NewContext(proj), db, dbPassword) != proj.Status.PasswordSyncHash {
		if err := r.ensureSyncPasswordJob(ctx, proj, db); err != nil {
			logger.Error(err, "Failed to ensure SyncPasswordJob")
			reconciler.SetNotReady(proj, "SyncPasswordJobFailed", err.Error())
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status after sync password job failure")
			}
			return ctrl.Result{}, err
		}

		syncPasswordJob := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.SyncPasswordJobName(project.NewContext(proj)), Namespace: proj.Namespace}, syncPasswordJob); err != nil {
			logger.Error(err, "Failed to get SyncPasswordJob")
			return ctrl.Result{}, err
		}
		if syncPasswordJob.Status.Failed > 0 {
			logger.Info("SyncPasswordJob failed")
			reconciler.SetNotReady(proj, "SyncPasswordJobFailed", "Sync password job failed")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status after sync password job failure")
			}
			return ctrl.Result{}, fmt.Errorf("sync-password job failed")
		}
		if syncPasswordJob.Status.Succeeded == 0 {
			logger.Info("Waiting for SyncPasswordJob to complete")
			reconciler.SetNotReady(proj, "SyncPasswordJobInProgress", "Waiting for sync password job to complete")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for sync password job")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}

		proj.Status.PasswordSyncHash = project.ComputePasswordSyncHash(project.NewContext(proj), db, dbPassword)
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update PasswordSyncHash status")
			return ctrl.Result{}, statusErr
		}
	}

	if err := r.markReady(ctx, proj); err != nil {
		logger.Error(err, "Failed to update Project status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureSyncJWTJob(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	job, err := project.SyncJWTJob(project.NewContext(proj), db)
	if err != nil {
		return fmt.Errorf("building sync-jwt job: %w", err)
	}
	if job == nil {
		return reconciler.DeleteJobIfExists(ctx, r.Client, project.SyncJWTJobName(project.NewContext(proj)), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", job.GetName(),
		"namespace", job.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, job, proj, reconciler.MutateJob())
	if err != nil {
		return fmt.Errorf("ensuring sync-jwt job: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created sync-jwt Job")
	case reconciler.ResultUpdated:
		logger.Info("Updated sync-jwt Job")
	default:
		logger.V(1).Info("sync-jwt Job unchanged")
	}

	return nil
}

func (r *Reconciler) ensureSyncPasswordJob(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	job, err := project.SyncPasswordJob(project.NewContext(proj), db)
	if err != nil {
		return fmt.Errorf("building sync-password job: %w", err)
	}
	if job == nil {
		return reconciler.DeleteJobIfExists(ctx, r.Client, project.SyncPasswordJobName(project.NewContext(proj)), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", job.GetName(),
		"namespace", job.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, job, proj, reconciler.MutateJob())
	if err != nil {
		return fmt.Errorf("ensuring sync-password job: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created sync-password Job")
	case reconciler.ResultUpdated:
		logger.Info("Updated sync-password Job")
	default:
		logger.V(1).Info("sync-password Job unchanged")
	}

	return nil
}

func (r *Reconciler) ensureJWTSecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.JWTSecret(project.NewContext(proj))
	if err != nil {
		return fmt.Errorf("building JWT secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.JWTSecretName(project.NewContext(proj)), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, proj, reconciler.MutateSecret(
		project.JWTSecretKey,
		project.JWTSecretAnonKey,
		project.JWTSecretServiceKey,
		project.JWTSecretKeys,
		project.JWTSecretJWKS,
		project.JWTSecretAnonKeyAsym,
		project.JWTSecretServiceKeyAsym,
		project.JWTSecretPublishableKey,
		project.JWTSecretOpaqueKey,
	))
	if err != nil {
		return fmt.Errorf("ensuring JWT secret: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created JWT Secret")
	case reconciler.ResultUpdated:
		logger.Info("Updated JWT Secret")
	default:
		logger.V(1).Info("JWT Secret unchanged")
	}

	return nil
}

func (r *Reconciler) ensureKeysSecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.KeysSecret(project.NewContext(proj))
	if err != nil {
		return fmt.Errorf("building keys secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.KeysSecretName(project.NewContext(proj)), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, proj, reconciler.MutateSecret(
		project.KeysSecretSecretKeyBase,
		project.KeysSecretCryptoKey,
		project.KeysSecretVaultEncKey,
		project.KeysSecretRealtimeDBEncKey,
	))
	if err != nil {
		return fmt.Errorf("ensuring keys secret: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Keys Secret")
	case reconciler.ResultUpdated:
		logger.Info("Updated Keys Secret")
	default:
		logger.V(1).Info("Keys Secret unchanged")
	}

	return nil
}

func (r *Reconciler) getJWTSecretValue(ctx context.Context, proj *supabasev1alpha1.Project) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: project.JWTSecretName(project.NewContext(proj)), Namespace: proj.Namespace}, secret); err != nil {
		return "", fmt.Errorf("getting JWT secret: %w", err)
	}
	value, ok := secret.Data[project.JWTSecretKey]
	if !ok {
		return "", fmt.Errorf("JWT secret key %q not found", project.JWTSecretKey)
	}
	return string(value), nil
}

func (r *Reconciler) getDBPasswordValue(ctx context.Context, namespace string, db *supabasev1alpha1.ResolvedDatabase) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: db.PasswordRef.Name, Namespace: namespace}, secret); err != nil {
		return "", fmt.Errorf("getting database password secret: %w", err)
	}
	value, ok := secret.Data[db.PasswordRef.Key]
	if !ok {
		return "", fmt.Errorf("database password secret key %q not found", db.PasswordRef.Key)
	}
	return string(value), nil
}

func (r *Reconciler) markReady(ctx context.Context, proj *supabasev1alpha1.Project) error {
	reconciler.SetReady(proj, "ReconcileSucceeded", "All resources reconciled successfully")
	return reconciler.UpdateStatus(ctx, r.Client, proj)
}

func (r *Reconciler) mapSecretToProjects(ctx context.Context, obj client.Object) []reconcile.Request {
	projects := &supabasev1alpha1.ProjectList{}
	if err := r.List(ctx, projects, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(projects.Items))
	for _, p := range projects.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&p)})
	}
	return requests
}
