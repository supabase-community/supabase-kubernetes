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
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
)

const ComponentDatabase = "database"

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

	if err := r.ensureSecret(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase secret")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeWarning, "SecretFailed", "SecretCreationFailed", "Failed to ensure secret: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "SecretFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after secret failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensurePVC(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase pvc")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeWarning, "PVCFailed", "PVCCreationFailed", "Failed to ensure PVC: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "PVCFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after pvc failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase service")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeWarning, "ServiceFailed", "ServiceCreationFailed", "Failed to ensure service: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "ServiceFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after service failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase statefulset")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeWarning, "StatefulSetFailed", "StatefulSetCreationFailed", "Failed to ensure StatefulSet: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after statefulset failure")
		}
		return ctrl.Result{}, err
	}

	singleDB.Status.ServiceName = singledatabase.ServiceName(singleDB.Name)
	singleDB.Status.SecretName = singledatabase.SecretName(singleDB.Name)
	r.populateStorageStatus(ctx, singleDB)

	// Check if the StatefulSet pod is actually ready
	sts := &appsv1.StatefulSet{}
	stsReady := false
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(singleDB.Name), Namespace: singleDB.Namespace}, sts); err == nil {
		if sts.Status.ReadyReplicas >= 1 {
			stsReady = true
		}
	}

	if stsReady {
		r.setCondition(singleDB, metav1.ConditionTrue, "AllResourcesReady", "Secret, Service and StatefulSet are ready")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeNormal, "Ready", "Ready", "Database pod is ready and accepting connections")
	} else {
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for database pod to become ready")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeWarning, "StatefulSetNotReady", "StatefulSetNotReady", "Waiting for database pod to become ready")
	}
	singleDB.Status.Phase = r.determinePhase(singleDB)

	if err := r.Status().Update(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to update SingleDatabase status")
		r.Recorder.Eventf(singleDB, nil, corev1.EventTypeWarning, "StatusUpdateFailed", "StatusUpdateFailed", "Failed to update status: %s", err.Error())
		return ctrl.Result{}, err
	}

	if !stsReady {
		logger.Info("Database pod not ready yet, requeuing")
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *SingleDatabaseReconciler) ensureSecret(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("secret", singledatabase.SecretName(singleDB.Name))

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: singledatabase.SecretName(singleDB.Name), Namespace: singleDB.Namespace}, secret)
	if err == nil {
		logger.V(1).Info("Secret already exists")
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting secret: %w", err)
	}

	logger.Info("Creating credentials secret")

	password, err := GenerateRandomAlphanumeric(32)
	if err != nil {
		return fmt.Errorf("generating postgres password: %w", err)
	}

	secret = singledatabase.BuildSecret(singleDB, password)

	if err := controllerutil.SetControllerReference(singleDB, secret, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on secret: %w", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}

	logger.Info("Created credentials secret")
	return nil
}

func (r *SingleDatabaseReconciler) ensurePVC(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("pvc", singledatabase.PVCName(singleDB.Name))

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: singledatabase.PVCName(singleDB.Name), Namespace: singleDB.Namespace}, pvc)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting pvc: %w", err)
		}

		logger.Info("Creating PVC")

		pvc = singledatabase.BuildPVC(singleDB)

		if singleDB.Spec.Storage.DeletionPolicy == supabasev1alpha1.PVCDeletionPolicyDelete || singleDB.Spec.Storage.DeletionPolicy == "" {
			if err := controllerutil.SetControllerReference(singleDB, pvc, r.Scheme); err != nil {
				return fmt.Errorf("setting owner reference on pvc: %w", err)
			}
		}

		if err := r.Create(ctx, pvc); err != nil {
			return fmt.Errorf("creating pvc: %w", err)
		}

		logger.Info("Created PVC")
		return nil
	}

	// PVC exists: check for updates
	needsUpdate := false

	// Only storage size can be expanded; accessModes and storageClassName are immutable.
	storageRes := singledatabase.StorageResources(singleDB)
	desiredStorage := storageRes.Requests.Storage()
	existingStorage := pvc.Spec.Resources.Requests.Storage()
	if desiredStorage != nil && (existingStorage == nil || desiredStorage.Cmp(*existingStorage) > 0) {
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{}
		}
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = *desiredStorage
		needsUpdate = true
		logger.Info("Expanding PVC storage", "from", existingStorage, "to", desiredStorage)
	}

	// Sync owner reference based on current pvcDeletionPolicy.
	ownerRefDesired := singleDB.Spec.Storage.DeletionPolicy == supabasev1alpha1.PVCDeletionPolicyDelete || singleDB.Spec.Storage.DeletionPolicy == ""
	ownerRefExists := false
	for _, ref := range pvc.OwnerReferences {
		if ref.UID == singleDB.UID {
			ownerRefExists = true
			break
		}
	}
	if ownerRefDesired && !ownerRefExists {
		if err := controllerutil.SetControllerReference(singleDB, pvc, r.Scheme); err != nil {
			return fmt.Errorf("setting owner reference on pvc: %w", err)
		}
		needsUpdate = true
		logger.Info("Adding owner reference to PVC")
	} else if !ownerRefDesired && ownerRefExists {
		newRefs := make([]metav1.OwnerReference, 0, len(pvc.OwnerReferences))
		for _, ref := range pvc.OwnerReferences {
			if ref.UID != singleDB.UID {
				newRefs = append(newRefs, ref)
			}
		}
		pvc.OwnerReferences = newRefs
		needsUpdate = true
		logger.Info("Removing owner reference from PVC")
	}

	if needsUpdate {
		if err := r.Update(ctx, pvc); err != nil {
			if r.isPVCResizeNotSupportedError(err) {
				logger.Info("PVC storage expansion is not supported by the storage provider", "error", err)
				return nil
			}
			return fmt.Errorf("updating pvc: %w", err)
		}
		logger.Info("Updated PVC")
	} else {
		logger.V(1).Info("PVC already exists and is up to date")
	}

	return nil
}

func (r *SingleDatabaseReconciler) ensureService(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("service", singledatabase.ServiceName(singleDB.Name))

	desired := singledatabase.BuildService(singleDB)

	if err := controllerutil.SetControllerReference(singleDB, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on service: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting service: %w", err)
		}
		logger.Info("Creating service")
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating service: %w", err)
		}
		logger.Info("Created service")
		return nil
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations
	logger.Info("Updating service")
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	return nil
}

func (r *SingleDatabaseReconciler) ensureStatefulSet(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("statefulset", singledatabase.StatefulSetName(singleDB.Name))

	image, err := r.resolveDatabaseImage(singleDB)
	if err != nil {
		return fmt.Errorf("resolving database image: %w", err)
	}
	secretName := singledatabase.SecretName(singleDB.Name)

	// Compute credential hash to trigger pod rollout on secret changes
	credentialHash, err := r.computeCredentialHash(ctx, singleDB.Namespace, secretName)
	if err != nil {
		return fmt.Errorf("computing credential hash: %w", err)
	}

	desired := singledatabase.BuildStatefulSet(singleDB, image, secretName, credentialHash)

	if err := controllerutil.SetControllerReference(singleDB, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on statefulset: %w", err)
	}

	existing := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting statefulset: %w", err)
		}
		logger.Info("Creating statefulset")
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating statefulset: %w", err)
		}
		logger.Info("Created statefulset")
		return nil
	}

	// Only update if there are actual changes to avoid unnecessary reconciliation loops
	needsUpdate := !reflect.DeepEqual(existing.Spec.Template, desired.Spec.Template) ||
		!reflect.DeepEqual(existing.Labels, desired.Labels) ||
		!reflect.DeepEqual(existing.Spec.Replicas, desired.Spec.Replicas)

	if !needsUpdate {
		logger.V(1).Info("StatefulSet already up to date")
		return nil
	}

	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels
	logger.Info("Updating statefulset")
	if err := r.Update(ctx, existing); err != nil {
		if apierrors.IsConflict(err) {
			// Conflict means object was modified concurrently; next reconcile will retry
			logger.V(1).Info("StatefulSet update conflict, will retry on next reconcile")
			return nil
		}
		return fmt.Errorf("updating statefulset: %w", err)
	}
	return nil
}

func (r *SingleDatabaseReconciler) populateStorageStatus(ctx context.Context, singleDB *supabasev1alpha1.SingleDatabase) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: singledatabase.PVCName(singleDB.Name), Namespace: singleDB.Namespace}, pvc); err == nil {
		if storage := pvc.Spec.Resources.Requests.Storage(); storage != nil {
			singleDB.Status.Storage = storage.String()
		}
	}
}

func (r *SingleDatabaseReconciler) determinePhase(singleDB *supabasev1alpha1.SingleDatabase) string {
	if meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady) {
		return "Running"
	}
	if len(singleDB.Status.Conditions) == 0 {
		return "Pending"
	}
	cond := meta.FindStatusCondition(singleDB.Status.Conditions, ConditionTypeReady)
	if cond != nil && cond.Status == metav1.ConditionFalse {
		if cond.Reason == "StatefulSetNotReady" {
			return "Creating"
		}
		return "Failed"
	}
	return "Creating"
}

func (r *SingleDatabaseReconciler) isPVCResizeNotSupportedError(err error) bool {
	if !apierrors.IsForbidden(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "resize") &&
		(strings.Contains(msg, "storageclass") || strings.Contains(msg, "dynamically provisioned"))
}

func (r *SingleDatabaseReconciler) computeCredentialHash(ctx context.Context, namespace, secretName string) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret); err != nil {
		return "", fmt.Errorf("getting secret for hash computation: %w", err)
	}

	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write(secret.Data[k])
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func (r *SingleDatabaseReconciler) resolveDatabaseImage(singleDB *supabasev1alpha1.SingleDatabase) (string, error) {
	if singleDB.Spec.Image != "" {
		return singleDB.Spec.Image, nil
	}
	return ResolveComponentImage(singleDB.Spec.Version, ComponentDatabase)
}

func (r *SingleDatabaseReconciler) setCondition(
	singleDB *supabasev1alpha1.SingleDatabase,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&singleDB.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             status,
		ObservedGeneration: singleDB.Generation,
		Reason:             reason,
		Message:            message,
	})
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
