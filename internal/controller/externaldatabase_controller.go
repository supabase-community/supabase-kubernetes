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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// ExternalDatabaseReconciler reconciles a ExternalDatabase object.
type ExternalDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=externaldatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=externaldatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=externaldatabases/finalizers,verbs=update

// Reconcile handles the reconciliation loop for ExternalDatabase resources.
func (r *ExternalDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	extDB := &platformv1alpha1.ExternalDatabase{}
	if err := r.Get(ctx, req.NamespacedName, extDB); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("ExternalDatabase resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ExternalDatabase")
		return ctrl.Result{}, err
	}

	if err := r.validate(extDB); err != nil {
		r.setCondition(extDB, ConditionTypeReady, metav1.ConditionFalse, "ValidationFailed", err.Error())
		if statusErr := r.Status().Update(ctx, extDB); statusErr != nil {
			logger.Error(statusErr, "Failed to update ExternalDatabase status after validation failure")
		}
		return ctrl.Result{}, err
	}

	r.setCondition(extDB, ConditionTypeReady, metav1.ConditionTrue, "ValidationSucceeded", "ExternalDatabase configuration is valid")
	if err := r.Status().Update(ctx, extDB); err != nil {
		logger.Error(err, "Failed to update ExternalDatabase status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *ExternalDatabaseReconciler) validate(extDB *platformv1alpha1.ExternalDatabase) error {
	if extDB.Spec.Host == "" {
		return fmt.Errorf("spec.host is required")
	}
	if extDB.Spec.PasswordRef.Name == "" {
		return fmt.Errorf("spec.passwordRef.name is required")
	}
	if extDB.Spec.PasswordRef.Key == "" {
		return fmt.Errorf("spec.passwordRef.key is required")
	}
	return nil
}

func (r *ExternalDatabaseReconciler) setCondition(
	extDB *platformv1alpha1.ExternalDatabase,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&extDB.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: extDB.Generation,
		Reason:             reason,
		Message:            message,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.ExternalDatabase{}).
		Named("externaldatabase").
		Complete(r)
}
