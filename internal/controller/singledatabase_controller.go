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
	"maps"
	"reflect"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	singleDatabaseComponent = "db"
	singleDatabasePort      = int32(5432)
	ComponentDatabase       = "database"
)

// SingleDatabaseReconciler reconciles a SingleDatabase object.
type SingleDatabaseReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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

	singleDB := &platformv1alpha1.SingleDatabase{}
	if err := r.Get(ctx, req.NamespacedName, singleDB); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("SingleDatabase resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SingleDatabase")
		return ctrl.Result{}, err
	}

	r.Recorder.Eventf(singleDB, corev1.EventTypeNormal, "Reconciling", "Starting reconciliation of SingleDatabase %s", singleDB.Name)

	if err := r.validate(singleDB); err != nil {
		logger.Error(err, "SingleDatabase validation failed")
		r.Recorder.Eventf(singleDB, corev1.EventTypeWarning, "ValidationFailed", "Validation failed: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "ValidationFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after validation failure")
		}
		return ctrl.Result{}, err
	}

	if _, err := r.ensureSecret(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase secret")
		r.Recorder.Eventf(singleDB, corev1.EventTypeWarning, "SecretFailed", "Failed to ensure secret: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "SecretFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after secret failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensurePVC(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase pvc")
		r.Recorder.Eventf(singleDB, corev1.EventTypeWarning, "PVCFailed", "Failed to ensure PVC: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "PVCFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after pvc failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase service")
		r.Recorder.Eventf(singleDB, corev1.EventTypeWarning, "ServiceFailed", "Failed to ensure service: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "ServiceFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after service failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to ensure SingleDatabase statefulset")
		r.Recorder.Eventf(singleDB, corev1.EventTypeWarning, "StatefulSetFailed", "Failed to ensure StatefulSet: %s", err.Error())
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetFailed", err.Error())
		if statusErr := r.Status().Update(ctx, singleDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update SingleDatabase status after statefulset failure")
		}
		return ctrl.Result{}, err
	}

	singleDB.Status.ServiceName = r.serviceName(singleDB.Name)
	singleDB.Status.SecretName = r.secretName(singleDB.Name)
	r.populateStorageStatus(ctx, singleDB)

	// Check if the StatefulSet pod is actually ready
	sts := &appsv1.StatefulSet{}
	stsReady := false
	if err := r.Get(ctx, types.NamespacedName{Name: r.statefulSetName(singleDB.Name), Namespace: singleDB.Namespace}, sts); err == nil {
		if sts.Status.ReadyReplicas >= 1 {
			stsReady = true
		}
	}

	if stsReady {
		r.setCondition(singleDB, metav1.ConditionTrue, "AllResourcesReady", "Secret, Service and StatefulSet are ready")
		r.Recorder.Event(singleDB, corev1.EventTypeNormal, "Ready", "Database pod is ready and accepting connections")
	} else {
		r.setCondition(singleDB, metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for database pod to become ready")
		r.Recorder.Event(singleDB, corev1.EventTypeWarning, "StatefulSetNotReady", "Waiting for database pod to become ready")
	}
	singleDB.Status.Phase = r.determinePhase(singleDB)

	if err := r.Status().Update(ctx, singleDB); err != nil {
		logger.Error(err, "Failed to update SingleDatabase status")
		r.Recorder.Eventf(singleDB, corev1.EventTypeWarning, "StatusUpdateFailed", "Failed to update status: %s", err.Error())
		return ctrl.Result{}, err
	}

	if !stsReady {
		logger.Info("Database pod not ready yet, requeuing")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *SingleDatabaseReconciler) populateStorageStatus(ctx context.Context, singleDB *platformv1alpha1.SingleDatabase) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: r.pvcName(singleDB.Name), Namespace: singleDB.Namespace}, pvc); err == nil {
		if storage := pvc.Spec.Resources.Requests.Storage(); storage != nil {
			singleDB.Status.Storage = storage.String()
		}
	}
}

func (r *SingleDatabaseReconciler) determinePhase(singleDB *platformv1alpha1.SingleDatabase) string {
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

func (r *SingleDatabaseReconciler) validate(singleDB *platformv1alpha1.SingleDatabase) error {
	return nil
}

func (r *SingleDatabaseReconciler) storageResources(singleDB *platformv1alpha1.SingleDatabase) corev1.VolumeResourceRequirements {
	if singleDB.Spec.Storage.Resources.Requests != nil || singleDB.Spec.Storage.Resources.Limits != nil {
		return singleDB.Spec.Storage.Resources
	}
	return corev1.VolumeResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("10Gi"),
		},
	}
}

func (r *SingleDatabaseReconciler) secretName(name string) string {
	return fmt.Sprintf("%s-db", name)
}

func (r *SingleDatabaseReconciler) serviceName(name string) string {
	return fmt.Sprintf("%s-%s", name, singleDatabaseComponent)
}

func (r *SingleDatabaseReconciler) statefulSetName(name string) string {
	return fmt.Sprintf("%s-%s", name, singleDatabaseComponent)
}

func (r *SingleDatabaseReconciler) ensureSecret(ctx context.Context, singleDB *platformv1alpha1.SingleDatabase) (bool, error) {
	logger := log.FromContext(ctx).WithValues("secret", r.secretName(singleDB.Name))

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: r.secretName(singleDB.Name), Namespace: singleDB.Namespace}, secret)
	if err == nil {
		logger.V(1).Info("Secret already exists")
		return false, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("getting secret: %w", err)
	}

	logger.Info("Creating credentials secret")

	password, err := GenerateRandomAlphanumeric(32)
	if err != nil {
		return false, fmt.Errorf("generating postgres password: %w", err)
	}

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.secretName(singleDB.Name),
			Namespace: singleDB.Namespace,
		},
		Data: map[string][]byte{
			"database": []byte("postgres"),
			"password": []byte(password),
		},
	}

	if err := controllerutil.SetControllerReference(singleDB, secret, r.Scheme); err != nil {
		return false, fmt.Errorf("setting owner reference on secret: %w", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return false, fmt.Errorf("creating secret: %w", err)
	}

	logger.Info("Created credentials secret")
	return true, nil
}

func (r *SingleDatabaseReconciler) pvcName(name string) string {
	return fmt.Sprintf("%s-%s-data", name, singleDatabaseComponent)
}

func (r *SingleDatabaseReconciler) ensurePVC(ctx context.Context, singleDB *platformv1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("pvc", r.pvcName(singleDB.Name))

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: r.pvcName(singleDB.Name), Namespace: singleDB.Namespace}, pvc)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting pvc: %w", err)
		}

		logger.Info("Creating PVC")

		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.pvcName(singleDB.Name),
				Namespace: singleDB.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      r.accessModes(singleDB.Spec.Storage.AccessModes),
				StorageClassName: singleDB.Spec.Storage.StorageClassName,
				Resources:        r.storageResources(singleDB),
			},
		}

		if singleDB.Spec.Storage.DeletionPolicy == platformv1alpha1.PVCDeletionPolicyDelete || singleDB.Spec.Storage.DeletionPolicy == "" {
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
	storageRes := r.storageResources(singleDB)
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
	ownerRefDesired := singleDB.Spec.Storage.DeletionPolicy == platformv1alpha1.PVCDeletionPolicyDelete || singleDB.Spec.Storage.DeletionPolicy == ""
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

func (r *SingleDatabaseReconciler) isPVCResizeNotSupportedError(err error) bool {
	if !apierrors.IsForbidden(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "resize") &&
		(strings.Contains(msg, "storageclass") || strings.Contains(msg, "dynamically provisioned"))
}

func (r *SingleDatabaseReconciler) ensureService(ctx context.Context, singleDB *platformv1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("service", r.serviceName(singleDB.Name))

	labels := map[string]string{
		"app.kubernetes.io/name":       "supabase",
		"app.kubernetes.io/instance":   singleDB.Name,
		"app.kubernetes.io/component":  singleDatabaseComponent,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
	annotations := map[string]string{}
	svcType := corev1.ServiceTypeClusterIP
	if singleDB.Spec.Service != nil {
		if singleDB.Spec.Service.Type != "" {
			svcType = singleDB.Spec.Service.Type
		}
		maps.Copy(annotations, singleDB.Spec.Service.Annotations)
		maps.Copy(labels, singleDB.Spec.Service.Labels)
	}

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.serviceName(singleDB.Name),
			Namespace:   singleDB.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: svcType,
			Selector: map[string]string{
				"app.kubernetes.io/name":      "supabase",
				"app.kubernetes.io/instance":  singleDB.Name,
				"app.kubernetes.io/component": singleDatabaseComponent,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Protocol:   corev1.ProtocolTCP,
					Port:       singleDatabasePort,
					TargetPort: intstr.FromInt32(singleDatabasePort),
				},
			},
		},
	}

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

// computeCredentialHash computes a deterministic hash of the credentials secret data.
// This is used as a pod template annotation to trigger pod rollouts when the secret changes.
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

func (r *SingleDatabaseReconciler) buildPasswordSyncInitContainer(image string, imagePullPolicy corev1.PullPolicy, secretName string) corev1.Container {
	return corev1.Container{
		Name:            "password-sync",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command:         []string{"sh", "-c", SingleDatabasePasswordSyncScript},
		Env: []corev1.EnvVar{
			r.envFromSecret("PGPASSWORD", secretName, "password"),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/var/lib/postgresql/data"},
		},
	}
}

func (r *SingleDatabaseReconciler) resolveDatabaseImage(singleDB *platformv1alpha1.SingleDatabase) (string, error) {
	if singleDB.Spec.Image != "" {
		return singleDB.Spec.Image, nil
	}
	return ResolveComponentImage(singleDB.Spec.Version, ComponentDatabase)
}

func (r *SingleDatabaseReconciler) ensureStatefulSet(ctx context.Context, singleDB *platformv1alpha1.SingleDatabase) error {
	logger := log.FromContext(ctx).WithValues("statefulset", r.statefulSetName(singleDB.Name))

	image, err := r.resolveDatabaseImage(singleDB)
	if err != nil {
		return fmt.Errorf("resolving database image: %w", err)
	}
	replicas := int32(1)
	secretName := r.secretName(singleDB.Name)

	// Compute credential hash to trigger pod rollout on secret changes
	credentialHash, err := r.computeCredentialHash(ctx, singleDB.Namespace, secretName)
	if err != nil {
		return fmt.Errorf("computing credential hash: %w", err)
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "supabase",
		"app.kubernetes.io/instance":   singleDB.Name,
		"app.kubernetes.io/component":  singleDatabaseComponent,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
	labels = r.mergeMaps(labels, singleDB.Spec.PodLabels)

	annotations := map[string]string{
		"supabase.io/secret-hash": credentialHash,
	}
	annotations = r.mergeMaps(annotations, singleDB.Spec.PodAnnotations)

	container := corev1.Container{
		Name:            singleDatabaseComponent,
		Image:           image,
		ImagePullPolicy: singleDB.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "postgres",
				ContainerPort: singleDatabasePort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			r.envFromSecret("POSTGRES_PASSWORD", secretName, "password"),
			r.envFromSecret("POSTGRES_DB", secretName, "database"),
			envVar("POSTGRES_HOST", "/var/run/postgresql"),
			envVar("POSTGRES_PORT", "5432"),
			r.envFromSecret("PGPASSWORD", secretName, "password"),
			envVar("PGPORT", "5432"),
			r.envFromSecret("PGDATABASE", secretName, "database"),
			envVar("PGHOST", "/var/run/postgresql"),
		},
		Resources:    singleDB.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/postgresql/data"}},
	}
	container.Env = append(container.Env, singleDB.Spec.Env...)

	if singleDB.Spec.ContainerSecurityContext != nil {
		container.SecurityContext = singleDB.Spec.ContainerSecurityContext
	}

	container.StartupProbe, container.ReadinessProbe, container.LivenessProbe = r.buildProbes(singleDB.Spec.Probes)

	initContainer := r.buildPasswordSyncInitContainer(image, singleDB.Spec.ImagePullPolicy, secretName)

	podSpec := corev1.PodSpec{
		InitContainers: []corev1.Container{initContainer},
		Containers:     []corev1.Container{container},
		Volumes: []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: r.pvcName(singleDB.Name),
					},
				},
			},
		},
		NodeSelector:                  singleDB.Spec.NodeSelector,
		Tolerations:                   singleDB.Spec.Tolerations,
		Affinity:                      singleDB.Spec.Affinity,
		PriorityClassName:             singleDB.Spec.PriorityClassName,
		TerminationGracePeriodSeconds: singleDB.Spec.TerminationGracePeriodSeconds,
	}

	if singleDB.Spec.PodSecurityContext != nil {
		podSpec.SecurityContext = singleDB.Spec.PodSecurityContext
	}

	desired := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.statefulSetName(singleDB.Name),
			Namespace: singleDB.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			ServiceName: r.serviceName(singleDB.Name),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}

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
	existing.Spec.UpdateStrategy = desired.Spec.UpdateStrategy
	existing.Spec.MinReadySeconds = desired.Spec.MinReadySeconds
	existing.Spec.RevisionHistoryLimit = desired.Spec.RevisionHistoryLimit
	existing.Spec.PersistentVolumeClaimRetentionPolicy = desired.Spec.PersistentVolumeClaimRetentionPolicy
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
func (r *SingleDatabaseReconciler) mergeMaps(base, override map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(override))
	maps.Copy(result, base)
	maps.Copy(result, override)
	return result
}

func (r *SingleDatabaseReconciler) buildProbes(probes *platformv1alpha1.ComponentProbes) (*corev1.Probe, *corev1.Probe, *corev1.Probe) {
	if probes != nil {
		return probes.Startup, probes.Readiness, probes.Liveness
	}

	pgIsReady := &corev1.ExecAction{
		Command: []string{"pg_isready", "-U", "postgres"},
	}

	startup := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    10,
		FailureThreshold: 30,
	}
	readiness := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    10,
		FailureThreshold: 3,
	}
	liveness := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    20,
		FailureThreshold: 3,
	}

	return startup, readiness, liveness
}

func (r *SingleDatabaseReconciler) accessModes(modes []corev1.PersistentVolumeAccessMode) []corev1.PersistentVolumeAccessMode {
	if len(modes) == 0 {
		return []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}
	return modes
}

func (r *SingleDatabaseReconciler) envFromSecret(name, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			},
		},
	}
}

func (r *SingleDatabaseReconciler) setCondition(
	singleDB *platformv1alpha1.SingleDatabase,
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
	r.Recorder = mgr.GetEventRecorderFor("singledatabase")
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.SingleDatabase{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.StatefulSet{}).
		Named("singledatabase").
		Complete(r)
}
