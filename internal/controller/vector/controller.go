package vector

import (
	"context"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	vectordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/vector"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const vectorFinalizer = "core.supabase.io/vector-rbac"

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=vectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=vectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=vectors/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;nodes;namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return component.SetupWithManager(mgr, r, component.SetupOptions{
		Object:  &core.Vector{},
		NewList: func() client.ObjectList { return &core.VectorList{} },
		Owns:    []client.Object{&appsv1.DaemonSet{}, &corev1.ConfigMap{}, &corev1.ServiceAccount{}},
		Related: []component.Object{
			&core.Analytics{}, &core.Auth{}, &core.Rest{}, &core.Realtime{},
			&core.Storage{}, &core.EdgeRuntime{}, &core.Envoy{},
		},
		NamespaceRelated: []client.Object{&rbacv1.ClusterRole{}, &rbacv1.ClusterRoleBinding{}},
	})
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &core.Vector{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, vectorFinalizer) {
			if err := r.cleanupRBAC(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, vectorFinalizer)
			return ctrl.Result{}, r.Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(obj, vectorFinalizer) {
		controllerutil.AddFinalizer(obj, vectorFinalizer)
		if err := r.Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
	}
	p, result, proceed, err := component.Start(ctx, r.Client, req, obj, &core.VectorList{}, "Vector")
	if err != nil || !proceed {
		return result, err
	}
	a, err := component.ListOptional(ctx, r.Client, obj.Namespace, p.Name, &core.AnalyticsList{}, "Analytics")
	if err != nil {
		return component.NotReady(ctx, r.Client, obj, "AmbiguousDependency", err.Error())
	}
	if a == nil || !a.GetDeletionTimestamp().IsZero() {
		if err := r.removeDaemonSet(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return component.NotReady(ctx, r.Client, obj, "DependencyNotFound", "Analytics is required; collection is stopped")
	}
	state := defaults.NewContext(p)
	state.Owner = obj
	state.Spec.Vector = &obj.Spec
	state.Names["Vector"] = obj.Name
	state.Names["Analytics"] = a.GetName()
	targets, err := r.targets(ctx, p)
	if err != nil {
		return component.NotReady(ctx, r.Client, obj, "DiscoveryFailed", err.Error())
	}
	if err := r.reconcileServiceAccount(ctx, state); err != nil {
		return component.NotReady(ctx, r.Client, obj, "RBACFailed", err.Error())
	}
	if err := r.reconcileRBAC(ctx, state, obj); err != nil {
		return component.NotReady(ctx, r.Client, obj, "RBACFailed", err.Error())
	}
	if err := r.reconcileConfigMap(ctx, state, targets); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ConfigurationFailed", err.Error())
	}
	if err := r.reconcileDaemonSet(ctx, state); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if !meta.IsStatusConditionTrue(a.(*core.Analytics).Status.Conditions, reconciler.ConditionTypeReady) {
		return component.NotReady(ctx, r.Client, obj, "AnalyticsNotReady", "Waiting for Analytics; collection retries delivery")
	}
	if err := component.DaemonSetReady(ctx, r.Client, client.ObjectKey{Namespace: state.Namespace, Name: vectordefaults.VectorName(state)}); err != nil {
		return component.NotReady(ctx, r.Client, obj, "WorkloadNotReady", err.Error())
	}
	return component.Ready(ctx, r.Client, obj)
}
