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
	"github.com/supabase-community/supabase-kubernetes/internal/images"
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

	r.Recorder.Eventf(singleDB, nil, corev1.EventTypeNormal, "Reconciling", "Reconciling", "Starting reconciliation of SingleDatabase %s", singleDB.Name)

	secretHash, err := r.ensureSecret(ctx, singleDB)
	if err != nil {
		logger.Error(err, "Failed to ensure Secret")
		r.setCondition(singleDB, metav1.ConditionFalse, "SecretFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after secret failure")
		}
		return ctrl.Result{}, err
	}

	configMapHash, err := r.ensureConfigMap(ctx, singleDB)
	if err != nil {
		logger.Error(err, "Failed to ensure ConfigMap")
		r.setCondition(singleDB, metav1.ConditionFalse, "ConfigMapFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after ConfigMap failure")
		}
		return ctrl.Result{}, err
	}

	image, err := r.resolveDatabaseImage(singleDB)
	if err != nil {
		logger.Error(err, "Failed to resolve database image")
		r.setCondition(singleDB, metav1.ConditionFalse, "ImageResolutionFailed", err.Error())
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after image resolution failure")
		}
		return ctrl.Result{}, err
	}

	secretName := singledatabase.SecretName(singleDB.Name)
	configMapName := singledatabase.ConfigMapName(singleDB.Name)

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

	if err := r.ensureStatefulSet(ctx, singleDB, image, secretName, configMapName, secretHash, configMapHash); err != nil {
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
		singleDB.Status.Phase = "Pending"
		if statusErr := r.updateStatus(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update status while waiting for StatefulSet")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	singleDB.Status.Phase = "Ready"
	singleDB.Status.ServiceName = singledatabase.ServiceName(singleDB.Name)
	singleDB.Status.SecretName = secretName
	singleDB.Status.Storage = r.resolveStorageStatus(singleDB)
	r.setCondition(singleDB, metav1.ConditionTrue, "ReconcileSucceeded", "All resources reconciled successfully")

	if err := r.updateStatus(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to update SingleDatabase status")
		return ctrl.Result{}, err
	}

	r.Recorder.Eventf(singleDB, nil, corev1.EventTypeNormal, "Reconciled", "Reconciled", "SingleDatabase %s is ready", singleDB.Name)
	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *SingleDatabaseReconciler) ensureSecret(ctx context.Context, db *supabasev1alpha1.SingleDatabase) (string, error) {
	password, err := helper.GenerateRandomAlphanumeric(32)
	if err != nil {
		return "", fmt.Errorf("generating password: %w", err)
	}

	desired := singledatabase.BuildSecret(db, password)
	mutateFn := func(existing, desired client.Object) error {
		e := existing.(*corev1.Secret)
		d := desired.(*corev1.Secret)
		if _, ok := e.Data[singledatabase.DefaultSecretPasswordKey]; ok {
			return nil
		}
		e.Data[singledatabase.DefaultSecretPasswordKey] = d.Data[singledatabase.DefaultSecretPasswordKey]
		return nil
	}
	obj, _, err := reconciler.EnsureResource(ctx, r.Client, desired, db, mutateFn)
	if err != nil {
		return "", fmt.Errorf("ensuring secret: %w", err)
	}

	r.Recorder.Eventf(db, nil, corev1.EventTypeNormal, "SecretCreated", "SecretCreated", "Created credentials Secret %s", singledatabase.SecretName(db.Name))
	return helper.SecretHash(obj.(*corev1.Secret)), nil
}

func (r *SingleDatabaseReconciler) ensurePVC(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	pvc := singledatabase.BuildPVC(db)
	mutateFn := func(existing, desired client.Object) error {
		e := existing.(*corev1.PersistentVolumeClaim)
		d := desired.(*corev1.PersistentVolumeClaim)
		e.Spec.Resources = d.Spec.Resources
		return nil
	}
	var owner client.Object = db
	if db.Spec.Storage.DeletionPolicy == supabasev1alpha1.PVCDeletionPolicyRetain {
		owner = nil
	}
	_, _, err := reconciler.EnsureResource(ctx, r.Client, pvc, owner, mutateFn)
	return err
}

func (r *SingleDatabaseReconciler) ensureService(ctx context.Context, db *supabasev1alpha1.SingleDatabase) error {
	svc := singledatabase.BuildService(db)
	mutateFn := func(existing, desired client.Object) error {
		e := existing.(*corev1.Service)
		d := desired.(*corev1.Service)
		e.Spec.Ports = d.Spec.Ports
		e.Spec.Selector = d.Spec.Selector
		e.Spec.Type = d.Spec.Type
		e.Labels = d.Labels
		e.Annotations = d.Annotations
		return nil
	}
	_, _, err := reconciler.EnsureResource(ctx, r.Client, svc, db, mutateFn)
	return err
}

func (r *SingleDatabaseReconciler) ensureConfigMap(ctx context.Context, db *supabasev1alpha1.SingleDatabase) (string, error) {
	cm := singledatabase.BuildConfigMap(db)
	mutateFn := func(existing, desired client.Object) error {
		e := existing.(*corev1.ConfigMap)
		d := desired.(*corev1.ConfigMap)
		e.Data = d.Data
		e.Labels = d.Labels
		e.Annotations = d.Annotations
		return nil
	}
	obj, _, err := reconciler.EnsureResource(ctx, r.Client, cm, db, mutateFn)
	if err != nil {
		return "", fmt.Errorf("ensuring configmap: %w", err)
	}

	r.Recorder.Eventf(db, nil, corev1.EventTypeNormal, "ConfigMapCreated", "ConfigMapCreated", "Created config %s", singledatabase.ConfigMapName(db.Name))
	return helper.ConfigMapHash(obj.(*corev1.ConfigMap)), nil
}

func (r *SingleDatabaseReconciler) ensureStatefulSet(ctx context.Context, db *supabasev1alpha1.SingleDatabase, image, secretName, configMapName, secretHash, configMapHash string) error {
	sts := singledatabase.BuildStatefulSet(db, image, secretName, configMapName, secretHash, configMapHash)
	mutateFn := func(existing, desired client.Object) error {
		e := existing.(*appsv1.StatefulSet)
		d := desired.(*appsv1.StatefulSet)
		e.Spec = d.Spec
		e.Labels = d.Labels
		e.Annotations = d.Annotations
		return nil
	}
	_, _, err := reconciler.EnsureResource(ctx, r.Client, sts, db, mutateFn)
	return err
}

func (r *SingleDatabaseReconciler) resolveDatabaseImage(db *supabasev1alpha1.SingleDatabase) (string, error) {
	if db.Spec.Image != "" {
		return db.Spec.Image, nil
	}
	return images.Resolve(db.Spec.Version, images.ComponentDatabase)
}

func (r *SingleDatabaseReconciler) resolveStorageStatus(db *supabasev1alpha1.SingleDatabase) string {
	if db.Spec.Storage.Resources.Requests != nil {
		if val, ok := db.Spec.Storage.Resources.Requests[corev1.ResourceStorage]; ok {
			return (&val).String()
		}
	}
	val := singledatabase.DefaultStorageResources().Requests[corev1.ResourceStorage]
	return (&val).String()
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
