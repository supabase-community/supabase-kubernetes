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
	"strconv"
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
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.StatefulSet{}).
		Named("singledatabase").
		Complete(r)
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=singledatabases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
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

	singleDB := &supabasev1alpha1.SingleDatabase{}
	if err := r.Get(ctx, req.NamespacedName, singleDB); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("SingleDatabase resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SingleDatabase")
		return ctrl.Result{}, err
	}

	r.defaultStorage(singleDB)

	if err := r.ensureSecret(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure Secret")
		reconciler.SetNotReady(singleDB, "SecretFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after secret failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureConfigMap(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure ConfigMap")
		reconciler.SetNotReady(singleDB, "ConfigMapFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after ConfigMap failure")
		}
		return ctrl.Result{}, err
	}

	sc := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.SecretName(singleDB.Name), Namespace: singleDB.Namespace}, sc); err != nil {
		logger.Error(err, "Failed to get Secret")
		return ctrl.Result{}, err
	}

	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.ConfigMapName(singleDB.Name), Namespace: singleDB.Namespace}, cm); err != nil {
		logger.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	secretHash := helper.SecretHash(sc)
	configMapHash := helper.ConfigMapHash(cm)

	if err := r.ensurePVC(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure PVC")
		reconciler.SetNotReady(singleDB, "PVCFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after PVC failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure Service")
		reconciler.SetNotReady(singleDB, "ServiceFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after Service failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, singleDB, secretHash, configMapHash); err != nil {
		logger.Error(err, "Failed to ensure StatefulSet")
		reconciler.SetNotReady(singleDB, "StatefulSetFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet failure")
		}
		return ctrl.Result{}, err
	}

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(singleDB.Name), Namespace: singleDB.Namespace}, sts); err != nil {
		logger.Error(err, "Failed to get StatefulSet")
		reconciler.SetNotReady(singleDB, "StatefulSetGetFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet get failure")
		}
		return ctrl.Result{}, err
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		logger.Info("Waiting for StatefulSet to be ready", "readyReplicas", sts.Status.ReadyReplicas, "replicas", *sts.Spec.Replicas)
		reconciler.SetNotReady(singleDB, "StatefulSetNotReady", "Waiting for StatefulSet pods to be ready")
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet not ready")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if err := r.markReady(ctx, singleDB, cm); err != nil {
		logger.Error(err, "Failed to update SingleDatabase status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *SingleDatabaseReconciler) ensureSecret(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	sc, err := singledatabase.BuildSecret(singleDB)
	if err != nil {
		return fmt.Errorf("building secret: %w", err)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", sc.GetName(),
		"namespace", sc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sc, singleDB, reconciler.MutateSecret(singledatabase.DefaultSecretPasswordKey))
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

func (r *SingleDatabaseReconciler) ensureConfigMap(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	cm := singledatabase.BuildConfigMap(singleDB)

	logger := log.FromContext(ctx).WithValues(
		"name", cm.GetName(),
		"namespace", cm.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, cm, singleDB, reconciler.MutateConfigMap())
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

func (r *SingleDatabaseReconciler) ensurePVC(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	pvc := singledatabase.BuildPVC(singleDB)
	var owner client.Object = singleDB
	if singleDB.Spec.Storage.DeletionPolicy != nil &&
		*singleDB.Spec.Storage.DeletionPolicy == supabasev1alpha1.DeletionPolicyRetain {
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

func (r *SingleDatabaseReconciler) ensureService(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	svc := singledatabase.BuildService(singleDB)

	logger := log.FromContext(ctx).WithValues(
		"name", svc.GetName(),
		"namespace", svc.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, svc, singleDB, reconciler.MutateService())
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

func (r *SingleDatabaseReconciler) ensureStatefulSet(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase, secretHash, configMapHash string) error {
	sts := singledatabase.BuildStatefulSet(singleDB, secretHash, configMapHash)

	logger := log.FromContext(ctx).WithValues(
		"name", sts.GetName(),
		"namespace", sts.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, sts, singleDB, reconciler.MutateStatefulSet())
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

func (r *SingleDatabaseReconciler) defaultStorage(singleDB *supabasev1alpha1.SingleDatabase) {
	if singleDB.Spec.Storage == nil {
		singleDB.Spec.Storage = singledatabase.DefaultStorage()
	}
}

func (r *SingleDatabaseReconciler) markReady(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase, configMap *corev1.ConfigMap) error {
	port, _ := strconv.Atoi(configMap.Data[singledatabase.DefaultConfigMapKeyPort])
	singleDB.Status.ResolvedDatabase = &supabasev1alpha1.ResolvedDatabase{
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local", singledatabase.ServiceName(singleDB.Name), singleDB.Namespace),
		Port:   int32(port),
		DBName: configMap.Data[singledatabase.DefaultConfigMapKeyDatabase],
		User:   configMap.Data[singledatabase.DefaultConfigMapKeyUser],
		PasswordRef: supabasev1alpha1.SecretKeyRef{
			Name: singledatabase.SecretName(singleDB.Name),
			Key:  singledatabase.DefaultSecretPasswordKey,
		},
	}

	reconciler.SetReady(singleDB, "ReconcileSucceeded", "All resources reconciled successfully")
	return reconciler.UpdateStatus(ctx, r.Client, singleDB)
}
