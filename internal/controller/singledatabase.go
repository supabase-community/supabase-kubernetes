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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
)

// SingleDatabaseReconciler reconciles a SingleDatabase object.
type SingleDatabaseReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *SingleDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&supabasev1alpha1.SingleDatabase{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.StatefulSet{}).
		Named("singledatabase").
		Complete(r)
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for SingleDatabase resources.
func (r *SingleDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"name", req.Name,
		"namespace", req.Namespace,
	)
	logger.Info("Starting SingleDatabase reconciliation")

	db := &supabasev1alpha1.SingleDatabase{}
	if err := r.Get(ctx, req.NamespacedName, db); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("SingleDatabase resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SingleDatabase")
		return ctrl.Result{}, err
	}

	if err := r.ensureSecret(ctx, db); err != nil {
		logger.Error(err, "Failed to ensure Secret")
		reconciler.SetNotReady(db, "SecretFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, db); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after secret failure")
		}
		return ctrl.Result{}, err
	}

	sc := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.PostgresSecretName(db), Namespace: db.Namespace}, sc); err != nil {
		logger.Error(err, "Failed to get Secret")
		return ctrl.Result{}, err
	}

	secretHash := helper.SecretHash(sc)

	if err := r.ensurePVC(ctx, db); err != nil {
		logger.Error(err, "Failed to ensure PVC")
		reconciler.SetNotReady(db, "PVCFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, db); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after PVC failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, db); err != nil {
		logger.Error(err, "Failed to ensure Service")
		reconciler.SetNotReady(db, "ServiceFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, db); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after Service failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, db, secretHash); err != nil {
		logger.Error(err, "Failed to ensure StatefulSet")
		reconciler.SetNotReady(db, "StatefulSetFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, db); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet failure")
		}
		return ctrl.Result{}, err
	}

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.PostgresStatefulSetName(db), Namespace: db.Namespace}, sts); err != nil {
		logger.Error(err, "Failed to get StatefulSet")
		reconciler.SetNotReady(db, "StatefulSetGetFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, db); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet get failure")
		}
		return ctrl.Result{}, err
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		logger.Info("Waiting for StatefulSet to be ready", "readyReplicas", sts.Status.ReadyReplicas, "replicas", *sts.Spec.Replicas)
		reconciler.SetNotReady(db, "StatefulSetNotReady", "Waiting for StatefulSet pods to be ready")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, db); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet not ready")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if err := r.markReady(ctx, db); err != nil {
		logger.Error(err, "Failed to update SingleDatabase status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *SingleDatabaseReconciler) ensureSecret(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	sc, err := singledatabase.PostgresSecret(db)
	if err != nil {
		return fmt.Errorf("building secret: %w", err)
	}
	if sc == nil {
		return reconciler.DeleteSecretIfExists(ctx, r.Client, singledatabase.PostgresSecretName(db), db.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, db, reconciler.MutateSecret(singledatabase.DefaultSecretKeyPassword))
	if err != nil {
		return fmt.Errorf("ensuring secret: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Secret")
	case reconciler.ResultUpdated:
		logger.Info("Updated Secret")
	default:
		logger.V(1).Info("Secret unchanged")
	}

	return nil
}

func (r *SingleDatabaseReconciler) ensurePVC(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	pvc, err := singledatabase.PostgresPVC(db)
	if err != nil {
		return fmt.Errorf("building pvc: %w", err)
	}
	if pvc == nil {
		return reconciler.DeletePersistentVolumeClaimIfExists(ctx, r.Client, singledatabase.PostgresPVCName(db), db.Namespace)
	}

	var owner client.Object = db
	if singledatabase.PostgresPVCDeletionPolicy(db) == supabasev1alpha1.DeletionPolicyRetain {
		owner = nil
	}

	logger := log.FromContext(ctx).WithValues(
		"name", pvc.GetName(),
		"namespace", pvc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, pvc, owner, reconciler.MutatePVC())
	if err != nil {
		return fmt.Errorf("ensuring pvc: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created PVC")
	case reconciler.ResultUpdated:
		logger.Info("Updated PVC")
	default:
		logger.V(1).Info("PVC unchanged")
	}

	return nil
}

func (r *SingleDatabaseReconciler) ensureService(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	svc, err := singledatabase.PostgresService(db)
	if err != nil {
		return fmt.Errorf("building service: %w", err)
	}
	if svc == nil {
		return reconciler.DeleteServiceIfExists(ctx, r.Client, singledatabase.PostgresServiceName(db), db.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, db, reconciler.MutateService())
	if err != nil {
		return fmt.Errorf("ensuring service: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created Service")
	case reconciler.ResultUpdated:
		logger.Info("Updated Service")
	default:
		logger.V(1).Info("Service unchanged")
	}

	return nil
}

func (r *SingleDatabaseReconciler) ensureStatefulSet(ctx context.Context, db *supabasev1alpha1.SingleDatabase, secretHash string) error {
	sts, err := singledatabase.PostgresStatefulSet(db, secretHash)
	if err != nil {
		return fmt.Errorf("building statefulset: %w", err)
	}
	if sts == nil {
		return reconciler.DeleteStatefulSetIfExists(ctx, r.Client, singledatabase.PostgresStatefulSetName(db), db.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sts.GetName(),
		"namespace", sts.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sts, db, reconciler.MutateStatefulSet())
	if err != nil {
		return fmt.Errorf("ensuring statefulset: %w", err)
	}

	switch result {
	case reconciler.ResultCreated:
		logger.Info("Created StatefulSet")
	case reconciler.ResultUpdated:
		logger.Info("Updated StatefulSet")
	default:
		logger.V(1).Info("StatefulSet unchanged")
	}

	return nil
}

func (r *SingleDatabaseReconciler) markReady(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	db.Status.ResolvedDatabase = &supabasev1alpha1.ResolvedDatabase{
		Host:   singledatabase.PostgresServiceHost(db),
		Port:   singledatabase.DefaultPostgresPort,
		DBName: singledatabase.DefaultPostgresDatabase,
		User:   singledatabase.DefaultPostgresUser,
		PasswordRef: supabasev1alpha1.SecretKeyRef{
			Name: singledatabase.PostgresSecretName(db),
			Key:  singledatabase.DefaultSecretKeyPassword,
		},
	}

	reconciler.SetReady(db, "ReconcileSucceeded", "All resources reconciled successfully")
	return reconciler.UpdateStatus(ctx, r.Client, db)
}
