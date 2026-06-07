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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/retry"
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
	logger := log.FromContext(ctx)

	singleDB := &supabasev1alpha1.SingleDatabase{}
	if err := r.Get(ctx, req.NamespacedName, singleDB); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("SingleDatabase resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SingleDatabase")
		return ctrl.Result{}, err
	}

	if err := r.ensureSecret(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure Secret")
		r.setCondition(singleDB, metav1.ConditionFalse, "SecretFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after secret failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureConfigMap(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure ConfigMap")
		r.setCondition(singleDB, metav1.ConditionFalse, "ConfigMapFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after ConfigMap failure")
		}
		return ctrl.Result{}, err
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.SecretName(singleDB.Name), Namespace: singleDB.Namespace}, secret); err != nil {
		logger.Error(err, "Failed to get Secret")
		return ctrl.Result{}, err
	}

	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.ConfigMapName(singleDB.Name), Namespace: singleDB.Namespace}, configMap); err != nil {
		logger.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	secretHash := helper.SecretHash(secret)
	configMapHash := helper.ConfigMapHash(configMap)

	if err := r.ensurePVC(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure PVC")
		r.setCondition(singleDB, metav1.ConditionFalse, "PVCFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after PVC failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure Service")
		r.setCondition(singleDB, metav1.ConditionFalse, "ServiceFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after Service failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, singleDB, secretHash, configMapHash); err != nil {
		logger.Error(err, "Failed to ensure StatefulSet")
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet failure")
		}
		return ctrl.Result{}, err
	}

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(singleDB.Name), Namespace: singleDB.Namespace}, sts); err != nil {
		logger.Error(err, "Failed to get StatefulSet")
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetGetFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after StatefulSet get failure")
		}
		return ctrl.Result{}, err
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		logger.Info("Waiting for StatefulSet to be ready", "readyReplicas", sts.Status.ReadyReplicas, "replicas", *sts.Spec.Replicas)
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for StatefulSet pods to be ready")
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while waiting for StatefulSet")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	if err := r.markReady(ctx, singleDB, configMap); err != nil {
		logger.Error(err, "Failed to update SingleDatabase status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *SingleDatabaseReconciler) ensureSecret(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	desired, err := singledatabase.BuildSecret(db)
	if err != nil {
		return fmt.Errorf("building secret: %w", err)
	}

	_, err = reconciler.EnsureResource(ctx, r.Client, desired, db, reconciler.MutateSecret(singledatabase.DefaultSecretPasswordKey))
	if err != nil {
		return fmt.Errorf("ensuring secret: %w", err)
	}

	return nil
}

func (r *SingleDatabaseReconciler) ensurePVC(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	pvc := singledatabase.BuildPVC(db)
	var owner client.Object = db
	if db.Spec.Storage.DeletionPolicy == supabasev1alpha1.PVCDeletionPolicyRetain {
		owner = nil
	}
	_, err := reconciler.EnsureResource(ctx, r.Client, pvc, owner, reconciler.MutatePVC())
	return err
}

func (r *SingleDatabaseReconciler) ensureService(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	svc := singledatabase.BuildService(db)
	_, err := reconciler.EnsureResource(ctx, r.Client, svc, db, reconciler.MutateService())
	return err
}

func (r *SingleDatabaseReconciler) ensureConfigMap(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	cm := singledatabase.BuildConfigMap(db)
	_, err := reconciler.EnsureResource(ctx, r.Client, cm, db, reconciler.MutateConfigMap())
	if err != nil {
		return fmt.Errorf("ensuring configmap: %w", err)
	}

	return nil
}

func (r *SingleDatabaseReconciler) ensureStatefulSet(ctx context.Context, db *supabasev1alpha1.SingleDatabase, secretHash, configMapHash string) error {
	sts := singledatabase.BuildStatefulSet(db, secretHash, configMapHash)
	_, err := reconciler.EnsureResource(ctx, r.Client, sts, db, reconciler.MutateStatefulSet())
	return err
}

func (r *SingleDatabaseReconciler) setCondition(
	db *supabasev1alpha1.SingleDatabase,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             status,
		ObservedGeneration: db.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func (r *SingleDatabaseReconciler) updateStatus(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &supabasev1alpha1.SingleDatabase{}
		if err := r.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, latest); err != nil {
			return err
		}
		latest.Status = db.Status
		return r.Status().Update(ctx, latest)
	})
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

	r.setCondition(singleDB, metav1.ConditionTrue, "ReconcileSucceeded", "All resources reconciled successfully")

	return r.updateStatus(ctx, singleDB)
}
