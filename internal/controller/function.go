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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/function"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

// FunctionReconciler reconciles a Function object.
type FunctionReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	RequeueInterval time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&supabasev1alpha1.Function{}).
		Owns(&corev1.ConfigMap{}).
		Named("function").
		Complete(r)
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for Function resources.
func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"name", req.Name,
		"namespace", req.Namespace,
	)
	logger.Info("Starting Function reconciliation")

	functionObj := &supabasev1alpha1.Function{}
	if err := r.Get(ctx, req.NamespacedName, functionObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Function resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Function")
		return ctrl.Result{}, err
	}

	if err := r.ensureConfigMap(ctx, functionObj); err != nil {
		logger.Error(err, "Failed to ensure ConfigMap")
		reconciler.SetNotReady(functionObj, "ConfigMapFailed", err.Error())
		if statusErr := reconciler.UpdateStatus(ctx, r.Client, functionObj); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after ConfigMap failure")
		}
		return ctrl.Result{}, err
	}

	if err := r.markReady(ctx, functionObj); err != nil {
		logger.Error(err, "Failed to update Function status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

func (r *FunctionReconciler) ensureConfigMap(ctx context.Context, functionObj *supabasev1alpha1.Function) error {
	cm, err := function.FunctionConfigMap(functionObj)
	if err != nil {
		return fmt.Errorf("building configmap: %w", err)
	}
	if cm == nil {
		return reconciler.DeleteConfigMapIfExists(ctx, r.Client, function.FunctionConfigMapName(functionObj), functionObj.Namespace)
	}

	logger := log.FromContext(ctx).WithValues(
		"name", cm.GetName(),
		"namespace", cm.GetNamespace(),
	)

	result, err := reconciler.EnsureResource(ctx, r.Client, cm, functionObj, reconciler.MutateConfigMap())
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

func (r *FunctionReconciler) markReady(ctx context.Context, functionObj *supabasev1alpha1.Function) error {
	reconciler.SetReady(functionObj, "ReconcileSucceeded", "ConfigMap reconciled successfully")
	return reconciler.UpdateStatus(ctx, r.Client, functionObj)
}
