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
	"maps"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	DefaultJWTExpiry = 24 * time.Hour * 365 * 10

	ConditionTypeSecretsReady = "SecretsReady"
	ConditionTypeReady        = "Ready"
)

type secretDefinition struct {
	suffix    string
	generator func() (SecretData, error)
}

// ProjectReconciler reconciles a Project object.
type ProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/status,verbs=get

// Reconcile handles the reconciliation loop for Project resources.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	project := &platformv1alpha1.Project{}
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
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after secret failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.EnsureAllComponents(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure components")
		r.setCondition(project, "ComponentsReady", metav1.ConditionFalse, "ComponentsFailed", err.Error())
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after component failure")
		}
		return ctrl.Result{}, err
	}
	r.setCondition(project, "ComponentsReady", metav1.ConditionTrue, "AllComponentsReady", "All enabled components are deployed")

	if err := r.reconcileHTTPRoute(ctx, project); err != nil {
		logger.Error(err, "Failed to reconcile HTTPRoute")
		r.setCondition(project, ConditionTypeReady, metav1.ConditionFalse, "HTTPRouteNotReady", err.Error())
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after HTTPRoute failure")
		}
		return ctrl.Result{}, err
	}

	r.setCondition(project, ConditionTypeSecretsReady, metav1.ConditionTrue, "AllSecretsReady", "All generated secrets are present and complete")
	r.setCondition(project, ConditionTypeReady, metav1.ConditionTrue, "ReconcileSucceeded", "All resources reconciled successfully")
	if err := r.Status().Update(ctx, project); err != nil {
		logger.Error(err, "Failed to update Project status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

// secretDefinitions returns the list of secrets that must exist for a Project.
func (r *ProjectReconciler) secretDefinitions() []secretDefinition {
	return []secretDefinition{
		{
			suffix: "jwt",
			generator: func() (SecretData, error) {
				return GenerateJWTSecretData(time.Now(), DefaultJWTExpiry)
			},
		},
		{suffix: "dashboard", generator: func() (SecretData, error) { return GenerateDashboardSecretData() }},
		{suffix: "keys", generator: func() (SecretData, error) { return GenerateKeysSecretData() }},
		{suffix: "storage-s3-protocol", generator: func() (SecretData, error) { return GenerateStorageS3SecretData() }},
	}
}

// ensureAllSecrets iterates over all secret definitions and ensures each one exists with all required keys.
func (r *ProjectReconciler) ensureAllSecrets(ctx context.Context, project *platformv1alpha1.Project) error {
	for _, def := range r.secretDefinitions() {
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
	owner *platformv1alpha1.Project,
	name string,
	generator func() (SecretData, error),
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
	project *platformv1alpha1.Project,
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

// keysOf returns the keys of a map as a string slice (for logging).
func keysOf(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	functionToProject := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		function, ok := obj.(*platformv1alpha1.Function)
		if !ok {
			return nil
		}
		if function.Spec.ProjectRef.Name == "" {
			return nil
		}
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name:      function.Spec.ProjectRef.Name,
				Namespace: function.Namespace,
			},
		}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Project{}).
		Watches(&platformv1alpha1.Function{}, functionToProject).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Owns(&platformv1alpha1.Function{}).
		Named("project").
		Complete(r)
}
