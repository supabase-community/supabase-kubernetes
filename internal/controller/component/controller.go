package component

import (
	"context"
	"fmt"
	"sort"
	"time"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ProjectRefIndex            = "spec.projectRef.name"
	ConflictingFunctionsReason = "ConflictingFunctions"
)

type Object interface {
	reconciler.ConditionedObject
	GetProjectRef() corev1.LocalObjectReference
}

type SetupOptions struct {
	Object         Object
	NewList        func() client.ObjectList
	Owns           []client.Object
	Related        []Object
	WatchFunctions bool
}

func SetupWithManager(mgr ctrl.Manager, r reconcile.Reconciler, options SetupOptions) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), options.Object, ProjectRefIndex, func(obj client.Object) []string {
		return []string{obj.(Object).GetProjectRef().Name}
	}); err != nil {
		return err
	}

	enqueueProject := enqueueForProject(mgr, options.NewList, func(obj client.Object) string { return obj.GetName() })
	enqueueNamespace := enqueueForProject(mgr, options.NewList, func(client.Object) string { return "" })
	enqueueReference := enqueueForProject(mgr, options.NewList, func(obj client.Object) string {
		return obj.(Object).GetProjectRef().Name
	})

	b := ctrl.NewControllerManagedBy(mgr).For(options.Object)
	for _, owned := range options.Owns {
		b = b.Owns(owned)
	}
	b = b.Watches(&core.Project{}, enqueueProject).
		Watches(&core.SingleDatabase{}, enqueueNamespace).
		Watches(&corev1.Secret{}, enqueueNamespace).
		Watches(&corev1.ConfigMap{}, enqueueNamespace)
	for _, related := range options.Related {
		b = b.Watches(related, enqueueReference)
	}
	if options.WatchFunctions {
		enqueueFunction := enqueueForProject(mgr, options.NewList, func(obj client.Object) string {
			return obj.(*core.Function).Spec.ProjectRef.Name
		})
		b = b.Watches(&core.Function{}, enqueueFunction)
	}
	return b.Complete(r)
}

func enqueueForProject(mgr ctrl.Manager, newList func() client.ObjectList, projectName func(client.Object) string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, changed client.Object) []reconcile.Request {
		list := newList()
		options := []client.ListOption{client.InNamespace(changed.GetNamespace())}
		if name := projectName(changed); name != "" {
			options = append(options, client.MatchingFields{ProjectRefIndex: name})
		}
		if err := mgr.GetClient().List(ctx, list, options...); err != nil {
			return nil
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return nil
		}
		requests := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(item.(client.Object))})
		}
		return requests
	})
}

// Start loads a component and its Project and rejects duplicate components.
func Start(ctx context.Context, c client.Client, req ctrl.Request, obj Object, list client.ObjectList, kind string) (*core.Project, ctrl.Result, bool, error) {
	if err := c.Get(ctx, req.NamespacedName, obj); err != nil {
		return nil, ctrl.Result{}, false, client.IgnoreNotFound(err)
	}
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil, ctrl.Result{}, false, nil
	}

	project := &core.Project{}
	err := c.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetProjectRef().Name}, project)
	if apierrors.IsNotFound(err) || (err == nil && !project.DeletionTimestamp.IsZero()) {
		result, updateErr := NotReady(ctx, c, obj, "ProjectNotFound", "Referenced Project is absent or deleting; dependent changes are suspended")
		return nil, result, false, updateErr
	}
	if err != nil {
		return nil, ctrl.Result{}, false, err
	}
	if err := c.List(ctx, list, client.InNamespace(obj.GetNamespace()), client.MatchingFields{ProjectRefIndex: project.Name}); err != nil {
		return nil, ctrl.Result{}, false, err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, ctrl.Result{}, false, err
	}
	if len(items) > 1 {
		result, updateErr := NotReady(ctx, c, obj, "ConflictingComponents", "Multiple "+kind+" resources reference this Project")
		return nil, result, false, updateErr
	}
	return project, ctrl.Result{}, true, nil
}

func ResolveDatabase(ctx context.Context, c client.Client, obj Object, project *core.Project) (*core.ResolvedDatabase, ctrl.Result, bool, error) {
	if !meta.IsStatusConditionTrue(project.Status.Conditions, reconciler.ConditionTypeReady) {
		result, err := NotReady(ctx, c, obj, "ProjectNotReady", "Shared Project infrastructure is not ready")
		return nil, result, false, err
	}
	db, ready, err := database.ResolveRef(ctx, c, project.Spec.DatabaseRef, project.Namespace)
	if err != nil {
		result, updateErr := NotReady(ctx, c, obj, "DatabaseResolutionFailed", err.Error())
		return nil, result, false, updateErr
	}
	if !ready {
		result, updateErr := NotReady(ctx, c, obj, "DatabaseNotReady", "Referenced database is not ready")
		return nil, result, false, updateErr
	}
	return db, ctrl.Result{}, true, nil
}

func RequireDependency(ctx context.Context, c client.Client, obj Object, list client.ObjectList, kind string) (client.Object, ctrl.Result, bool, error) {
	if err := c.List(ctx, list, client.InNamespace(obj.GetNamespace()), client.MatchingFields{ProjectRefIndex: obj.GetProjectRef().Name}); err != nil {
		return nil, ctrl.Result{}, false, err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, ctrl.Result{}, false, err
	}
	if len(items) == 0 {
		result, updateErr := NotReady(ctx, c, obj, "DependencyNotFound", kind+" resource is required")
		return nil, result, false, updateErr
	}
	if len(items) > 1 {
		result, updateErr := NotReady(ctx, c, obj, "AmbiguousDependency", "Multiple "+kind+" resources reference this Project")
		return nil, result, false, updateErr
	}
	return items[0].(client.Object), ctrl.Result{}, true, nil
}

func ListOptional(ctx context.Context, c client.Client, namespace, projectName string, list client.ObjectList, kind string) (client.Object, error) {
	if err := c.List(ctx, list, client.InNamespace(namespace), client.MatchingFields{ProjectRefIndex: projectName}); err != nil {
		return nil, err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("multiple %s resources reference this Project", kind)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0].(client.Object), nil
}

func ListFunctions(ctx context.Context, c client.Client, namespace, projectName string) ([]core.Function, error) {
	list := &core.FunctionList{}
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("listing functions: %w", err)
	}
	functions := make([]core.Function, 0, len(list.Items))
	for _, function := range list.Items {
		if function.Spec.ProjectRef.Name == projectName {
			functions = append(functions, function)
		}
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].Name < functions[j].Name })
	return functions, nil
}

func ValidateFunctions(functions []core.Function, owner client.Object, claimMain bool) (string, error) {
	names := map[string]bool{}
	for i := range functions {
		function := &functions[i]
		if names[function.Spec.FunctionName] {
			return ConflictingFunctionsReason, fmt.Errorf("duplicate logical function name: %s", function.Spec.FunctionName)
		}
		names[function.Spec.FunctionName] = true
		if claimMain && function.Spec.FunctionName == "main" && !metav1.IsControlledBy(function, owner) {
			return ConflictingFunctionsReason, fmt.Errorf("logical function main is already claimed")
		}
		if !meta.IsStatusConditionTrue(function.Status.Conditions, reconciler.ConditionTypeReady) {
			return "FunctionNotReady", fmt.Errorf("waiting for Function %s", function.Name)
		}
	}
	return "", nil
}

func NotReady(ctx context.Context, c client.Client, obj Object, reason, message string) (ctrl.Result, error) {
	reconciler.SetNotReady(obj, reason, message)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, reconciler.UpdateStatus(ctx, c, obj)
}

func Ready(ctx context.Context, c client.Client, obj Object) (ctrl.Result, error) {
	reconciler.SetReady(obj, "ReconcileSucceeded", "Component resources reconciled successfully")
	return ctrl.Result{}, reconciler.UpdateStatus(ctx, c, obj)
}
