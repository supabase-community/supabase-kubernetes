package studio

import (
	"context"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	studiodefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/studio"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=studios,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=studios/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=studios/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &core.Studio{}
	project, result, proceed, err := component.Start(ctx, r.Client, req, obj, &core.StudioList{}, "Studio")
	if err != nil || !proceed {
		return result, err
	}
	related, result, proceed, err := component.RequireDependency(ctx, r.Client, obj, &core.MetaList{}, "Meta")
	if err != nil || !proceed {
		return result, err
	}
	meta := related.(*core.Meta)
	related, result, proceed, err = component.RequireDependency(ctx, r.Client, obj, &core.EnvoyList{}, "Envoy")
	if err != nil || !proceed {
		return result, err
	}
	envoy := related.(*core.Envoy)
	db, result, proceed, err := component.ResolveDatabase(ctx, r.Client, obj, project)
	if err != nil || !proceed {
		return result, err
	}
	functions, err := component.ListFunctions(ctx, r.Client, obj.Namespace, project.Name)
	if err != nil {
		return component.NotReady(ctx, r.Client, obj, "FunctionsFetchFailed", err.Error())
	}
	if reason, err := component.ValidateFunctions(functions, obj, false); err != nil {
		return component.NotReady(ctx, r.Client, obj, reason, err.Error())
	}
	state := defaults.NewContext(project)
	state.Owner = obj
	state.Spec.Studio = &obj.Spec
	state.Names["Studio"] = obj.Name
	state.Spec.Meta = &meta.Spec
	state.Names["Meta"] = meta.Name
	state.Spec.Envoy = &envoy.Spec
	state.Names["Envoy"] = envoy.Name
	for _, reconcile := range []func(context.Context, *defaults.Context) error{r.reconcilePVC, r.reconcileService} {
		if err := reconcile(ctx, state); err != nil {
			return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
		}
	}
	if err := r.reconcileStatefulSet(ctx, state, db); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := component.StatefulSetReady(ctx, r.Client, client.ObjectKey{Namespace: obj.Namespace, Name: studiodefaults.StudioStatefulSetName(state)}); err != nil {
		return component.NotReady(ctx, r.Client, obj, "WorkloadNotReady", err.Error())
	}
	return component.Ready(ctx, r.Client, obj)
}
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return component.SetupWithManager(mgr, r, component.SetupOptions{Object: &core.Studio{}, NewList: func() client.ObjectList { return &core.StudioList{} }, Owns: []client.Object{&appsv1.StatefulSet{}, &corev1.Service{}, &corev1.PersistentVolumeClaim{}, &corev1.ServiceAccount{}, &rbacv1.Role{}, &rbacv1.RoleBinding{}}, Related: []component.Object{&core.Meta{}, &core.Envoy{}}, WatchFunctions: true})
}
