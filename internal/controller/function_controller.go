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
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// FunctionReconciler reconciles a Function object
type FunctionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=functions/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.supabase.io,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

const (
	conditionTypeSourceReady = "SourceReady"
	conditionTypeReady       = "Ready"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Function object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	function := &platformv1alpha1.Function{}
	if err := r.Get(ctx, req.NamespacedName, function); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	function.Status.ObservedGeneration = function.Generation
	function.Status.TargetProject = function.Spec.ProjectRef.Name

	project := &platformv1alpha1.Project{}
	projectKey := client.ObjectKey{Name: function.Spec.ProjectRef.Name, Namespace: function.Namespace}
	if err := r.Get(ctx, projectKey, project); err != nil {
		if apierrors.IsNotFound(err) {
			r.setFunctionCondition(function, conditionTypeSourceReady, metav1.ConditionFalse, "ProjectNotFound", "Target Project was not found")
			r.setFunctionCondition(function, conditionTypeReady, metav1.ConditionFalse, "ProjectNotFound", "Target Project was not found")
			if updateErr := r.Status().Update(ctx, function); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if err := validateFunctionSource(function.Spec.Source); err != nil {
		r.setFunctionCondition(function, conditionTypeSourceReady, metav1.ConditionFalse, "InvalidSource", err.Error())
		r.setFunctionCondition(function, conditionTypeReady, metav1.ConditionFalse, "InvalidSource", err.Error())
		if updateErr := r.Status().Update(ctx, function); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	if function.Spec.FunctionName == "main" {
		msg := "Function name 'main' is reserved for the built-in runtime entrypoint"
		r.setFunctionCondition(function, conditionTypeSourceReady, metav1.ConditionFalse, "ReservedFunctionName", msg)
		r.setFunctionCondition(function, conditionTypeReady, metav1.ConditionFalse, "ReservedFunctionName", msg)
		if updateErr := r.Status().Update(ctx, function); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	duplicate, err := r.hasDuplicateFunctionName(ctx, function)
	if err != nil {
		return ctrl.Result{}, err
	}
	if duplicate {
		msg := "Another Function already uses this functionName for the same project"
		r.setFunctionCondition(function, conditionTypeSourceReady, metav1.ConditionFalse, "DuplicateFunctionName", msg)
		r.setFunctionCondition(function, conditionTypeReady, metav1.ConditionFalse, "DuplicateFunctionName", msg)
		if updateErr := r.Status().Update(ctx, function); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	if err := r.ensureFunctionCodeConfigMap(ctx, function); err != nil {
		logger.Error(err, "Failed to ensure Function ConfigMap")
		return ctrl.Result{}, err
	}

	function.Status.ConfigMapName = supabaseFunctionCodeConfigMapName(function)
	r.setFunctionCondition(function, conditionTypeSourceReady, metav1.ConditionTrue, "SourceValidated", "Source files are valid")
	r.setFunctionCondition(function, conditionTypeReady, metav1.ConditionTrue, "ConfigMapReady", "Function source was synced to ConfigMap")
	if err := r.Status().Update(ctx, function); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully", "functionName", function.Spec.FunctionName, "project", function.Spec.ProjectRef.Name)
	return ctrl.Result{}, nil
}

func validateFunctionSource(source map[string]string) error {
	if len(source) == 0 {
		return fmt.Errorf("source must include at least one file")
	}
	indexSource, ok := source["index.ts"]
	if !ok {
		return fmt.Errorf("source must include index.ts")
	}
	if strings.TrimSpace(indexSource) == "" {
		return fmt.Errorf("source index.ts must not be empty")
	}

	for fileName := range source {
		if strings.TrimSpace(fileName) == "" {
			return fmt.Errorf("source file name must not be empty")
		}
		if strings.Contains(fileName, "/") {
			return fmt.Errorf("source file %q must be in root folder", fileName)
		}
	}

	return nil
}

func supabaseFunctionCodeConfigMapName(function *platformv1alpha1.Function) string {
	return function.Name + "-code"
}

func sortedSource(source map[string]string) map[string]string {
	keys := make([]string, 0, len(source))
	for k := range source {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make(map[string]string, len(source))
	for _, key := range keys {
		result[key] = source[key]
	}
	return result
}

func (r *FunctionReconciler) ensureFunctionCodeConfigMap(ctx context.Context, function *platformv1alpha1.Function) error {
	name := supabaseFunctionCodeConfigMapName(function)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: function.Namespace}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(function, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = map[string]string{
			"app.kubernetes.io/name":       "supabase",
			"app.kubernetes.io/managed-by": "supabase-operator",
			"app.kubernetes.io/component":  "functions-source",
			"supabase.project":             function.Spec.ProjectRef.Name,
			"supabase.function":            function.Spec.FunctionName,
		}
		cm.Data = sortedSource(function.Spec.Source)
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensuring Function ConfigMap: %w", err)
	}
	return nil
}

func (r *FunctionReconciler) hasDuplicateFunctionName(ctx context.Context, function *platformv1alpha1.Function) (bool, error) {
	list := &platformv1alpha1.FunctionList{}
	if err := r.List(ctx, list, client.InNamespace(function.Namespace)); err != nil {
		return false, fmt.Errorf("listing Functions: %w", err)
	}

	for i := range list.Items {
		item := &list.Items[i]
		if item.Name == function.Name {
			continue
		}
		if item.Spec.ProjectRef.Name == function.Spec.ProjectRef.Name && item.Spec.FunctionName == function.Spec.FunctionName {
			return true, nil
		}
	}

	return false, nil
}

func (r *FunctionReconciler) setFunctionCondition(
	function *platformv1alpha1.Function,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&function.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: function.Generation,
		Reason:             reason,
		Message:            message,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	projectToFunctions := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		project, ok := obj.(*platformv1alpha1.Project)
		if !ok {
			return nil
		}

		functionList := &platformv1alpha1.FunctionList{}
		if err := r.List(ctx, functionList, client.InNamespace(project.Namespace)); err != nil {
			return nil
		}

		requests := make([]reconcile.Request, 0)
		for i := range functionList.Items {
			fn := functionList.Items[i]
			if fn.Spec.ProjectRef.Name != project.Name {
				continue
			}
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&fn)})
		}

		return requests
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Function{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&platformv1alpha1.Project{}, projectToFunctions).
		Named("function").
		Complete(r)
}
