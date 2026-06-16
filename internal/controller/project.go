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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	"github.com/supabase-community/supabase-kubernetes/internal/project"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

// ProjectReconciler reconciles a Project object.
type ProjectReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&supabasev1alpha1.Project{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.Deployment{}).
		Owns(&batchv1.Job{}).
		Owns(&supabasev1alpha1.Migration{}).
		Owns(&supabasev1alpha1.Function{}).
		Watches(
			&supabasev1alpha1.SingleDatabase{},
			handler.EnqueueRequestsFromMapFunc(r.mapSingleDatabaseToProjects),
		).
		Watches(
			&supabasev1alpha1.Function{},
			handler.EnqueueRequestsFromMapFunc(r.mapFunctionToProject),
		).
		Named("project").
		Complete(r)
}

func (r *ProjectReconciler) mapSingleDatabaseToProjects(ctx context.Context, obj client.Object) []reconcile.Request {
	singleDB, ok := obj.(*supabasev1alpha1.SingleDatabase)
	if !ok {
		return nil
	}

	projects := &supabasev1alpha1.ProjectList{}
	if err := r.List(ctx, projects); err != nil {
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

func (r *ProjectReconciler) mapFunctionToProject(ctx context.Context, obj client.Object) []reconcile.Request {
	functionObj, ok := obj.(*supabasev1alpha1.Function)
	if !ok {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      functionObj.Spec.ProjectRef,
				Namespace: functionObj.Namespace,
			},
		},
	}
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
// +kubebuilder:rbac:groups=core.supabase.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=migrations/status,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/status,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for Project resources.
//
//nolint:gocyclo // keep the reconciliation steps explicit
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	if err := r.ensureMigration1(ctx, proj); err != nil {
		logger.Error(err, "Failed to ensure Migration")
		reconciler.SetNotReady(proj, "MigrationFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after migration failure")
		}
		return ctrl.Result{}, err
	}

	migration := &supabasev1alpha1.Migration{}
	if err := r.Get(ctx, types.NamespacedName{Name: project.ProjectMigration1Name(proj), Namespace: proj.Namespace}, migration); err != nil {
		logger.Error(err, "Failed to get Migration")
		return ctrl.Result{}, err
	}
	if !meta.IsStatusConditionTrue(migration.Status.Conditions, reconciler.ConditionTypeReady) {
		logger.Info("Waiting for Migration to be ready")
		reconciler.SetNotReady(proj, "MigrationNotReady", "Waiting for migration to be ready")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while waiting for migration")
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

	if project.ComputeJWTSyncHash(proj, db, dbPassword, jwtSecret) != proj.Status.JwtSyncHash {
		if err := r.ensureSyncJWTJob(ctx, proj, db); err != nil {
			logger.Error(err, "Failed to ensure SyncJWTJob")
			reconciler.SetNotReady(proj, "SyncJWTJobFailed", err.Error())
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status after sync JWT job failure")
			}
			return ctrl.Result{}, err
		}

		syncJWTJob := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.SyncJWTJobName(proj), Namespace: proj.Namespace}, syncJWTJob); err != nil {
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

		proj.Status.JwtSyncHash = project.ComputeJWTSyncHash(proj, db, dbPassword, jwtSecret)
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update JwtSyncHash status")
			return ctrl.Result{}, statusErr
		}
	}

	if project.ComputePasswordSyncHash(proj, db, dbPassword) != proj.Status.PasswordSyncHash {
		if err := r.ensureSyncPasswordJob(ctx, proj, db); err != nil {
			logger.Error(err, "Failed to ensure SyncPasswordJob")
			reconciler.SetNotReady(proj, "SyncPasswordJobFailed", err.Error())
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status after sync password job failure")
			}
			return ctrl.Result{}, err
		}

		syncPasswordJob := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.SyncPasswordJobName(proj), Namespace: proj.Namespace}, syncPasswordJob); err != nil {
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

		proj.Status.PasswordSyncHash = project.ComputePasswordSyncHash(proj, db, dbPassword)
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update PasswordSyncHash status")
			return ctrl.Result{}, statusErr
		}
	}

	if err := r.ensureMainFunction(ctx, proj); err != nil {
		logger.Error(err, "Failed to ensure main Function")
		reconciler.SetNotReady(proj, "MainFunctionFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after main Function failure")
		}
		return ctrl.Result{}, err
	}

	if proj.Spec.Functions != nil && *proj.Spec.Functions.Enable {
		mainFunction := &supabasev1alpha1.Function{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.ProjectMainFunctionName(proj), Namespace: proj.Namespace}, mainFunction); err != nil {
			logger.Error(err, "Failed to get main Function")
			return ctrl.Result{}, err
		}
		if !meta.IsStatusConditionTrue(mainFunction.Status.Conditions, reconciler.ConditionTypeReady) {
			logger.Info("Waiting for main Function to be ready")
			reconciler.SetNotReady(proj, "MainFunctionNotReady", "Waiting for main function to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for main Function")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	functions, err := r.fetchProjectFunctions(ctx, proj)
	if err != nil {
		logger.Error(err, "Failed to fetch Functions")
		reconciler.SetNotReady(proj, "FunctionsFetchFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after functions fetch failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureFunctions(ctx, proj, db, functions); err != nil {
		logger.Error(err, "Failed to ensure Functions component")
		reconciler.SetNotReady(proj, "FunctionsFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after functions failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureAuth(ctx, proj, db); err != nil {
		logger.Error(err, "Failed to ensure Auth component")
		reconciler.SetNotReady(proj, "AuthFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after auth failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureRest(ctx, proj, db); err != nil {
		logger.Error(err, "Failed to ensure Rest component")
		reconciler.SetNotReady(proj, "RestFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after rest failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureMeta(ctx, proj, db); err != nil {
		logger.Error(err, "Failed to ensure Meta component")
		reconciler.SetNotReady(proj, "MetaFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after meta failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureRealtime(ctx, proj, db); err != nil {
		logger.Error(err, "Failed to ensure Realtime component")
		reconciler.SetNotReady(proj, "RealtimeFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after realtime failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStorage(ctx, proj, db); err != nil {
		logger.Error(err, "Failed to ensure Storage component")
		reconciler.SetNotReady(proj, "StorageFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after storage failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureEnvoy(ctx, proj); err != nil {
		logger.Error(err, "Failed to ensure Envoy component")
		reconciler.SetNotReady(proj, "EnvoyFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after envoy failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStudio(ctx, proj, db, functions); err != nil {
		logger.Error(err, "Failed to ensure Studio component")
		reconciler.SetNotReady(proj, "StudioFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after studio failure")
		}
		return ctrl.Result{}, err
	}

	if proj.Spec.Auth != nil && *proj.Spec.Auth.Enable {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.AuthDeploymentName(proj), Namespace: proj.Namespace}, deploy); err != nil {
			logger.Error(err, "Failed to get Auth Deployment")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Auth Deployment to be ready", "readyReplicas", deploy.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "DeploymentsNotReady", "Waiting for deployments to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for deployments")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Rest != nil && *proj.Spec.Rest.Enable {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.RestDeploymentName(proj), Namespace: proj.Namespace}, deploy); err != nil {
			logger.Error(err, "Failed to get Rest Deployment")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Rest Deployment to be ready", "readyReplicas", deploy.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "DeploymentsNotReady", "Waiting for deployments to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for deployments")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Meta != nil && *proj.Spec.Meta.Enable {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.MetaDeploymentName(proj), Namespace: proj.Namespace}, deploy); err != nil {
			logger.Error(err, "Failed to get Meta Deployment")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Meta Deployment to be ready", "readyReplicas", deploy.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "DeploymentsNotReady", "Waiting for deployments to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for deployments")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Realtime != nil && *proj.Spec.Realtime.Enable {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.RealtimeDeploymentName(proj), Namespace: proj.Namespace}, deploy); err != nil {
			logger.Error(err, "Failed to get Realtime Deployment")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Realtime Deployment to be ready", "readyReplicas", deploy.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "DeploymentsNotReady", "Waiting for deployments to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for deployments")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Functions != nil && *proj.Spec.Functions.Enable {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.FunctionsDeploymentName(proj), Namespace: proj.Namespace}, deploy); err != nil {
			logger.Error(err, "Failed to get Functions Deployment")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Functions Deployment to be ready", "readyReplicas", deploy.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "DeploymentsNotReady", "Waiting for deployments to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for deployments")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Envoy != nil && *proj.Spec.Envoy.Enable {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.EnvoyDeploymentName(proj), Namespace: proj.Namespace}, deploy); err != nil {
			logger.Error(err, "Failed to get Envoy Deployment")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Envoy Deployment to be ready", "readyReplicas", deploy.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "DeploymentsNotReady", "Waiting for deployments to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for deployments")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Storage != nil && *proj.Spec.Storage.Enable {
		sts := &appsv1.StatefulSet{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.StorageStatefulSetName(proj), Namespace: proj.Namespace}, sts); err != nil {
			logger.Error(err, "Failed to get Storage StatefulSet")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		if sts.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Storage StatefulSet to be ready", "readyReplicas", sts.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "StatefulSetNotReady", "Waiting for Storage StatefulSet to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for Storage StatefulSet")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if proj.Spec.Studio != nil && *proj.Spec.Studio.Enable {
		sts := &appsv1.StatefulSet{}
		if err := r.Get(ctx, types.NamespacedName{Name: project.StudioStatefulSetName(proj), Namespace: proj.Namespace}, sts); err != nil {
			logger.Error(err, "Failed to get Studio StatefulSet")
			return ctrl.Result{}, err
		}
		replicas := int32(1)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		if sts.Status.ReadyReplicas < replicas {
			logger.Info("Waiting for Studio StatefulSet to be ready", "readyReplicas", sts.Status.ReadyReplicas, "replicas", replicas)
			reconciler.SetNotReady(proj, "StatefulSetNotReady", "Waiting for Studio StatefulSet to be ready")
			if statusErr := reconciler.UpdateStatus(ctx, r.Client, proj); statusErr != nil {
				logger.Error(statusErr, "Failed to update status while waiting for Studio StatefulSet")
			}
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}

	if err := r.markReady(ctx, proj); err != nil {
		logger.Error(err, "Failed to update Project status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) ensureMigration1(ctx context.Context, proj *supabasev1alpha1.Project) error {
	migration, err := project.ProjectMigration1(proj)
	if err != nil {
		return fmt.Errorf("building migration: %w", err)
	}
	if migration == nil {
		return reconciler.DeleteMigrationIfExists(ctx, r.Client, project.ProjectMigration1Name(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", migration.GetName(),
		"namespace", migration.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, migration, proj, reconciler.MutateMigration())
	if err != nil {
		return fmt.Errorf("ensuring migration: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Migration")
	case reconciler.ResultUpdated:
		logger.Info("Updated Migration")
	default:
		logger.V(1).Info("Migration unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureMainFunction(ctx context.Context, proj *supabasev1alpha1.Project) error {
	functionObj, err := project.ProjectMainFunction(proj)
	if err != nil {
		return fmt.Errorf("building main function: %w", err)
	}
	if functionObj == nil {
		return reconciler.DeleteFunctionIfExists(ctx, r.Client, project.ProjectMainFunctionName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", functionObj.GetName(),
		"namespace", functionObj.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, functionObj, proj, reconciler.MutateFunction())
	if err != nil {
		return fmt.Errorf("ensuring main function: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created main Function")
	case reconciler.ResultUpdated:
		logger.Info("Updated main Function")
	default:
		logger.V(1).Info("main Function unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureSyncJWTJob(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	job, err := project.SyncJWTJob(proj, db)
	if err != nil {
		return fmt.Errorf("building sync-jwt job: %w", err)
	}
	if job == nil {
		return reconciler.DeleteJobIfExists(ctx, r.Client, project.SyncJWTJobName(proj), proj.Namespace)
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

func (r *ProjectReconciler) ensureSyncPasswordJob(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	job, err := project.SyncPasswordJob(proj, db)
	if err != nil {
		return fmt.Errorf("building sync-password job: %w", err)
	}
	if job == nil {
		return reconciler.DeleteJobIfExists(ctx, r.Client, project.SyncPasswordJobName(proj), proj.Namespace)
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

func (r *ProjectReconciler) ensureJWTSecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.JWTSecret(proj)
	if err != nil {
		return fmt.Errorf("building JWT secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.JWTSecretName(proj), proj.Namespace)
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

func (r *ProjectReconciler) ensureKeysSecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.KeysSecret(proj)
	if err != nil {
		return fmt.Errorf("building keys secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.KeysSecretName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, proj, reconciler.MutateSecret(
		project.KeysSecretSecretKeyBase,
		project.KeysSecretCryptoKey,
		project.KeysSecretVaultEncKey,
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

func (r *ProjectReconciler) ensureAuthSecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.AuthSecret(proj)
	if err != nil {
		return fmt.Errorf("building auth secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.AuthSecretName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, proj, reconciler.MutateSecret(project.AuthSecretSAMLPrivateKey))
	if err != nil {
		return fmt.Errorf("ensuring auth secret: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Auth Secret")
	case reconciler.ResultUpdated:
		logger.Info("Updated Auth Secret")
	default:
		logger.V(1).Info("Auth Secret unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureAuth(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	if err := r.ensureAuthSecret(ctx, proj); err != nil {
		return fmt.Errorf("ensuring auth secret: %w", err)
	}
	if err := r.ensureAuthService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring auth service: %w", err)
	}
	if err := r.ensureAuthDeployment(ctx, proj, db); err != nil {
		return fmt.Errorf("ensuring auth deployment: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureRest(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	if err := r.ensureRestService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring rest service: %w", err)
	}
	if err := r.ensureRestDeployment(ctx, proj, db); err != nil {
		return fmt.Errorf("ensuring rest deployment: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureMeta(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	if err := r.ensureMetaService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring meta service: %w", err)
	}
	if err := r.ensureMetaDeployment(ctx, proj, db); err != nil {
		return fmt.Errorf("ensuring meta deployment: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureRealtime(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	if err := r.ensureRealtimeService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring realtime service: %w", err)
	}
	if err := r.ensureRealtimeDeployment(ctx, proj, db); err != nil {
		return fmt.Errorf("ensuring realtime deployment: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureAuthService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.AuthService(proj)
	if err != nil {
		return fmt.Errorf("building auth service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.AuthServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring auth service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Auth Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Auth Service")
	default:
		logger.V(1).Info("Auth Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureAuthDeployment(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	deploy, err := project.AuthDeployment(proj, db)
	if err != nil {
		return fmt.Errorf("building auth deployment: %w", err)
	}
	if deploy == nil {
		return reconciler.DeleteDeploymentIfExists(ctx, r.Client, project.AuthDeploymentName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", deploy.GetName(),
		"namespace", deploy.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, deploy, proj, reconciler.MutateDeployment())
	if err != nil {
		return fmt.Errorf("ensuring auth deployment: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Auth Deployment")
	case reconciler.ResultUpdated:
		logger.Info("Updated Auth Deployment")
	default:
		logger.V(1).Info("Auth Deployment unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureRestService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.RestService(proj)
	if err != nil {
		return fmt.Errorf("building rest service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.RestServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring rest service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Rest Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Rest Service")
	default:
		logger.V(1).Info("Rest Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureRestDeployment(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	deploy, err := project.RestDeployment(proj, db)
	if err != nil {
		return fmt.Errorf("building rest deployment: %w", err)
	}
	if deploy == nil {
		return reconciler.DeleteDeploymentIfExists(ctx, r.Client, project.RestDeploymentName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", deploy.GetName(),
		"namespace", deploy.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, deploy, proj, reconciler.MutateDeployment())
	if err != nil {
		return fmt.Errorf("ensuring rest deployment: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Rest Deployment")
	case reconciler.ResultUpdated:
		logger.Info("Updated Rest Deployment")
	default:
		logger.V(1).Info("Rest Deployment unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureMetaService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.MetaService(proj)
	if err != nil {
		return fmt.Errorf("building meta service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.MetaServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring meta service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Meta Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Meta Service")
	default:
		logger.V(1).Info("Meta Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureMetaDeployment(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	deploy, err := project.MetaDeployment(proj, db)
	if err != nil {
		return fmt.Errorf("building meta deployment: %w", err)
	}
	if deploy == nil {
		return reconciler.DeleteDeploymentIfExists(ctx, r.Client, project.MetaDeploymentName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", deploy.GetName(),
		"namespace", deploy.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, deploy, proj, reconciler.MutateDeployment())
	if err != nil {
		return fmt.Errorf("ensuring meta deployment: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Meta Deployment")
	case reconciler.ResultUpdated:
		logger.Info("Updated Meta Deployment")
	default:
		logger.V(1).Info("Meta Deployment unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureRealtimeService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.RealtimeService(proj)
	if err != nil {
		return fmt.Errorf("building realtime service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.RealtimeServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring realtime service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Realtime Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Realtime Service")
	default:
		logger.V(1).Info("Realtime Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureRealtimeDeployment(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	deploy, err := project.RealtimeDeployment(proj, db)
	if err != nil {
		return fmt.Errorf("building realtime deployment: %w", err)
	}
	if deploy == nil {
		return reconciler.DeleteDeploymentIfExists(ctx, r.Client, project.RealtimeDeploymentName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", deploy.GetName(),
		"namespace", deploy.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, deploy, proj, reconciler.MutateDeployment())
	if err != nil {
		return fmt.Errorf("ensuring realtime deployment: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Realtime Deployment")
	case reconciler.ResultUpdated:
		logger.Info("Updated Realtime Deployment")
	default:
		logger.V(1).Info("Realtime Deployment unchanged")
	}

	return nil
}

func (r *ProjectReconciler) fetchProjectFunctions(ctx context.Context, proj *supabasev1alpha1.Project) ([]supabasev1alpha1.Function, error) {
	functionList := &supabasev1alpha1.FunctionList{}
	if err := r.List(ctx, functionList, client.InNamespace(proj.Namespace)); err != nil {
		return nil, fmt.Errorf("listing functions: %w", err)
	}

	var functions []supabasev1alpha1.Function
	for _, f := range functionList.Items {
		if f.Spec.ProjectRef == proj.Name {
			functions = append(functions, f)
		}
	}
	return functions, nil
}

func (r *ProjectReconciler) ensureFunctions(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, functions []supabasev1alpha1.Function) error {
	if err := r.ensureFunctionsService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring functions service: %w", err)
	}
	if err := r.ensureFunctionsDeployment(ctx, proj, db, functions); err != nil {
		return fmt.Errorf("ensuring functions deployment: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureFunctionsService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.FunctionsService(proj)
	if err != nil {
		return fmt.Errorf("building functions service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.FunctionsServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring functions service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Functions Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Functions Service")
	default:
		logger.V(1).Info("Functions Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureFunctionsDeployment(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, functions []supabasev1alpha1.Function) error {
	deploy, err := project.FunctionsDeployment(proj, functions, db)
	if err != nil {
		return fmt.Errorf("building functions deployment: %w", err)
	}
	if deploy == nil {
		return reconciler.DeleteDeploymentIfExists(ctx, r.Client, project.FunctionsDeploymentName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", deploy.GetName(),
		"namespace", deploy.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, deploy, proj, reconciler.MutateDeployment())
	if err != nil {
		return fmt.Errorf("ensuring functions deployment: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Functions Deployment")
	case reconciler.ResultUpdated:
		logger.Info("Updated Functions Deployment")
	default:
		logger.V(1).Info("Functions Deployment unchanged")
	}

	return nil
}

func (r *ProjectReconciler) getJWTSecretValue(ctx context.Context, proj *supabasev1alpha1.Project) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: project.JWTSecretName(proj), Namespace: proj.Namespace}, secret); err != nil {
		return "", fmt.Errorf("getting JWT secret: %w", err)
	}
	value, ok := secret.Data[project.JWTSecretKey]
	if !ok {
		return "", fmt.Errorf("JWT secret key %q not found", project.JWTSecretKey)
	}
	return string(value), nil
}

func (r *ProjectReconciler) getDBPasswordValue(ctx context.Context, namespace string, db *supabasev1alpha1.ResolvedDatabase) (string, error) {
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

func (r *ProjectReconciler) ensureStorage(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	if err := r.ensureStorageSecret(ctx, proj); err != nil {
		return fmt.Errorf("ensuring storage secret: %w", err)
	}
	if err := r.ensureStoragePVC(ctx, proj); err != nil {
		return fmt.Errorf("ensuring storage pvc: %w", err)
	}
	if err := r.ensureStorageService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring storage service: %w", err)
	}
	if err := r.ensureStorageStatefulSet(ctx, proj, db); err != nil {
		return fmt.Errorf("ensuring storage statefulset: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureStorageSecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.StorageSecret(proj)
	if err != nil {
		return fmt.Errorf("building storage secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.StorageSecretName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, proj, reconciler.MutateSecret(
		project.StorageSecretAccessKeyID,
		project.StorageSecretAccessKeySecret,
	))
	if err != nil {
		return fmt.Errorf("ensuring storage secret: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Storage Secret")
	case reconciler.ResultUpdated:
		logger.Info("Updated Storage Secret")
	default:
		logger.V(1).Info("Storage Secret unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureStoragePVC(ctx context.Context, proj *supabasev1alpha1.Project) error {
	pvc, err := project.StoragePVC(proj)
	if err != nil {
		return fmt.Errorf("building storage pvc: %w", err)
	}
	if pvc == nil {
		return reconciler.DeletePersistentVolumeClaimIfExists(ctx, r.Client, project.StoragePVCName(proj), proj.Namespace)
	}

	var owner client.Object = proj
	if project.StoragePVCDeletionPolicy(proj) == supabasev1alpha1.DeletionPolicyRetain {
		owner = nil
	}

	logger := log.FromContext(ctx).WithValues(
		"name", pvc.GetName(),
		"namespace", pvc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, pvc, owner, reconciler.MutatePVC())
	if err != nil {
		return fmt.Errorf("ensuring storage pvc: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Storage PVC")
	case reconciler.ResultUpdated:
		logger.Info("Updated Storage PVC")
	default:
		logger.V(1).Info("Storage PVC unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureStorageService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.StorageService(proj)
	if err != nil {
		return fmt.Errorf("building storage service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.StorageServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring storage service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Storage Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Storage Service")
	default:
		logger.V(1).Info("Storage Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureStorageStatefulSet(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	sts, err := project.StorageStatefulSet(proj, db)
	if err != nil {
		return fmt.Errorf("building storage statefulset: %w", err)
	}
	if sts == nil {
		return reconciler.DeleteStatefulSetIfExists(ctx, r.Client, project.StorageStatefulSetName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sts.GetName(),
		"namespace", sts.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sts, proj, reconciler.MutateStatefulSet())
	if err != nil {
		return fmt.Errorf("ensuring storage statefulset: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Storage StatefulSet")
	case reconciler.ResultUpdated:
		logger.Info("Updated Storage StatefulSet")
	default:
		logger.V(1).Info("Storage StatefulSet unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureEnvoy(ctx context.Context, proj *supabasev1alpha1.Project) error {
	if err := r.ensureEnvoySecret(ctx, proj); err != nil {
		return fmt.Errorf("ensuring envoy secret: %w", err)
	}

	envoySecret, err := r.getEnvoySecret(ctx, proj)
	if err != nil {
		return fmt.Errorf("getting envoy secret: %w", err)
	}
	secretHash := helper.SecretHash(envoySecret)

	if err := r.ensureEnvoyConfigMap(ctx, proj); err != nil {
		return fmt.Errorf("ensuring envoy configmap: %w", err)
	}

	envoyConfigMap, err := r.getEnvoyConfigMap(ctx, proj)
	if err != nil {
		return fmt.Errorf("getting envoy configmap: %w", err)
	}
	configMapHash := helper.ConfigMapHash(envoyConfigMap)

	if err := r.ensureEnvoyService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring envoy service: %w", err)
	}
	if err := r.ensureEnvoyDeployment(ctx, proj, configMapHash, secretHash); err != nil {
		return fmt.Errorf("ensuring envoy deployment: %w", err)
	}

	return nil
}

func (r *ProjectReconciler) ensureEnvoySecret(ctx context.Context, proj *supabasev1alpha1.Project) error {
	sc, err := project.EnvoySecret(proj)
	if err != nil {
		return fmt.Errorf("building envoy secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, project.EnvoySecretName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, proj, reconciler.MutateSecret(
		project.DefaultEnvoySecretKeyUsername,
		project.DefaultEnvoySecretKeyPassword,
	))
	if err != nil {
		return fmt.Errorf("ensuring envoy secret: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Envoy Secret")
	case reconciler.ResultUpdated:
		logger.Info("Updated Envoy Secret")
	default:
		logger.V(1).Info("Envoy Secret unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureEnvoyConfigMap(ctx context.Context, proj *supabasev1alpha1.Project) error {
	cm, err := project.EnvoyConfigMap(proj)
	if err != nil {
		return fmt.Errorf("building envoy configmap: %w", err)
	}
	if cm == nil {
		return reconciler.DeleteConfigMapIfExists(ctx, r.Client, project.EnvoyConfigMapName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", cm.GetName(),
		"namespace", cm.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, cm, proj, reconciler.MutateConfigMap())
	if err != nil {
		return fmt.Errorf("ensuring envoy configmap: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Envoy ConfigMap")
	case reconciler.ResultUpdated:
		logger.Info("Updated Envoy ConfigMap")
	default:
		logger.V(1).Info("Envoy ConfigMap unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureEnvoyService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.EnvoyService(proj)
	if err != nil {
		return fmt.Errorf("building envoy service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.EnvoyServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring envoy service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Envoy Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Envoy Service")
	default:
		logger.V(1).Info("Envoy Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureEnvoyDeployment(ctx context.Context, proj *supabasev1alpha1.Project, configMapHash, secretHash string) error {
	deploy, err := project.EnvoyDeployment(proj, configMapHash, secretHash)
	if err != nil {
		return fmt.Errorf("building envoy deployment: %w", err)
	}
	if deploy == nil {
		return reconciler.DeleteDeploymentIfExists(ctx, r.Client, project.EnvoyDeploymentName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", deploy.GetName(),
		"namespace", deploy.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, deploy, proj, reconciler.MutateDeployment())
	if err != nil {
		return fmt.Errorf("ensuring envoy deployment: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Envoy Deployment")
	case reconciler.ResultUpdated:
		logger.Info("Updated Envoy Deployment")
	default:
		logger.V(1).Info("Envoy Deployment unchanged")
	}

	return nil
}

func (r *ProjectReconciler) getEnvoySecret(ctx context.Context, proj *supabasev1alpha1.Project) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: project.EnvoySecretName(proj), Namespace: proj.Namespace}, secret); err != nil {
		return nil, fmt.Errorf("getting envoy secret: %w", err)
	}
	return secret, nil
}

func (r *ProjectReconciler) ensureStudio(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, functions []supabasev1alpha1.Function) error {
	if err := r.ensureStudioPVC(ctx, proj); err != nil {
		return fmt.Errorf("ensuring studio pvc: %w", err)
	}
	if err := r.ensureStudioService(ctx, proj); err != nil {
		return fmt.Errorf("ensuring studio service: %w", err)
	}
	if err := r.ensureStudioStatefulSet(ctx, proj, db, functions); err != nil {
		return fmt.Errorf("ensuring studio statefulset: %w", err)
	}
	return nil
}

func (r *ProjectReconciler) ensureStudioPVC(ctx context.Context, proj *supabasev1alpha1.Project) error {
	pvc, err := project.StudioPVC(proj)
	if err != nil {
		return fmt.Errorf("building studio pvc: %w", err)
	}
	if pvc == nil {
		return reconciler.DeletePersistentVolumeClaimIfExists(ctx, r.Client, project.StudioPVCName(proj), proj.Namespace)
	}

	var owner client.Object = proj
	if project.StudioPVCDeletionPolicy(proj) == supabasev1alpha1.DeletionPolicyRetain {
		owner = nil
	}

	logger := log.FromContext(ctx).WithValues(
		"name", pvc.GetName(),
		"namespace", pvc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, pvc, owner, reconciler.MutatePVC())
	if err != nil {
		return fmt.Errorf("ensuring studio pvc: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Studio PVC")
	case reconciler.ResultUpdated:
		logger.Info("Updated Studio PVC")
	default:
		logger.V(1).Info("Studio PVC unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureStudioService(ctx context.Context, proj *supabasev1alpha1.Project) error {
	svc, err := project.StudioService(proj)
	if err != nil {
		return fmt.Errorf("building studio service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, project.StudioServiceName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, proj, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring studio service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Studio Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Studio Service")
	default:
		logger.V(1).Info("Studio Service unchanged")
	}

	return nil
}

func (r *ProjectReconciler) ensureStudioStatefulSet(ctx context.Context, proj *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, functions []supabasev1alpha1.Function) error {
	sts, err := project.StudioStatefulSet(proj, functions, db)
	if err != nil {
		return fmt.Errorf("building studio statefulset: %w", err)
	}
	if sts == nil {
		return reconciler.DeleteStatefulSetIfExists(ctx, r.Client, project.StudioStatefulSetName(proj), proj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sts.GetName(),
		"namespace", sts.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sts, proj, reconciler.MutateStatefulSet())
	if err != nil {
		return fmt.Errorf("ensuring studio statefulset: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Studio StatefulSet")
	case reconciler.ResultUpdated:
		logger.Info("Updated Studio StatefulSet")
	default:
		logger.V(1).Info("Studio StatefulSet unchanged")
	}

	return nil
}

func (r *ProjectReconciler) getEnvoyConfigMap(ctx context.Context, proj *supabasev1alpha1.Project) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: project.EnvoyConfigMapName(proj), Namespace: proj.Namespace}, cm); err != nil {
		return nil, fmt.Errorf("getting envoy configmap: %w", err)
	}
	return cm, nil
}

func (r *ProjectReconciler) markReady(ctx context.Context, proj *supabasev1alpha1.Project) error {
	reconciler.SetReady(proj, "ReconcileSucceeded", "All resources reconciled successfully")
	return reconciler.UpdateStatus(ctx, r.Client, proj)
}
