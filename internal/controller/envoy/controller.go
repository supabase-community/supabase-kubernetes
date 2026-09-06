package envoy

import (
	"context"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	envoydefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/envoy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=envoys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=envoys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=envoys/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &core.Envoy{}
	project, result, proceed, err := component.Start(ctx, r.Client, req, obj, &core.EnvoyList{}, "Envoy")
	if err != nil || !proceed {
		return result, err
	}
	state := defaults.NewContext(project)
	state.Owner = obj
	state.Spec.Envoy = &obj.Spec
	state.Names["Envoy"] = obj.Name
	dependencies := []struct {
		list  client.ObjectList
		kind  string
		apply func(client.Object)
	}{
		{&core.AuthList{}, "Auth", func(found client.Object) {
			value := found.(*core.Auth)
			state.Spec.Auth = &value.Spec
			state.Names["Auth"] = value.Name
		}},
		{&core.RestList{}, "Rest", func(found client.Object) {
			value := found.(*core.Rest)
			state.Spec.Rest = &value.Spec
			state.Names["Rest"] = value.Name
		}},
		{&core.MetaList{}, "Meta", func(found client.Object) {
			value := found.(*core.Meta)
			state.Spec.Meta = &value.Spec
			state.Names["Meta"] = value.Name
		}},
		{&core.RealtimeList{}, "Realtime", func(found client.Object) {
			value := found.(*core.Realtime)
			state.Spec.Realtime = &value.Spec
			state.Names["Realtime"] = value.Name
		}},
		{&core.StorageList{}, "Storage", func(found client.Object) {
			value := found.(*core.Storage)
			state.Spec.Storage = &value.Spec
			state.Names["Storage"] = value.Name
		}},
		{&core.StudioList{}, "Studio", func(found client.Object) {
			value := found.(*core.Studio)
			state.Spec.Studio = &value.Spec
			state.Names["Studio"] = value.Name
		}},
		{&core.EdgeRuntimeList{}, "EdgeRuntime", func(found client.Object) {
			value := found.(*core.EdgeRuntime)
			state.Spec.EdgeRuntime = &value.Spec
			state.Names["EdgeRuntime"] = value.Name
		}},
	}
	for _, dependency := range dependencies {
		found, err := component.ListOptional(ctx, r.Client, obj.Namespace, project.Name, dependency.list, dependency.kind)
		if err != nil {
			return component.NotReady(ctx, r.Client, obj, "AmbiguousDependency", err.Error())
		}
		if found != nil {
			dependency.apply(found)
		}
	}
	if _, result, proceed, err = component.ResolveDatabase(ctx, r.Client, obj, project); err != nil || !proceed {
		return result, err
	}
	secretHash, err := r.reconcileSecret(ctx, state)
	if err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	configHash, err := r.reconcileConfigMap(ctx, state)
	if err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := r.reconcileService(ctx, state); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := r.reconcileDeployment(ctx, state, configHash, secretHash); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := component.DeploymentReady(ctx, r.Client, client.ObjectKey{Namespace: obj.Namespace, Name: envoydefaults.EnvoyDeploymentName(state)}); err != nil {
		return component.NotReady(ctx, r.Client, obj, "WorkloadNotReady", err.Error())
	}
	return component.Ready(ctx, r.Client, obj)
}
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return component.SetupWithManager(mgr, r, component.SetupOptions{Object: &core.Envoy{}, NewList: func() client.ObjectList { return &core.EnvoyList{} }, Owns: []client.Object{&appsv1.Deployment{}, &corev1.Service{}, &corev1.Secret{}, &corev1.ConfigMap{}}, Related: []component.Object{&core.Auth{}, &core.Rest{}, &core.Meta{}, &core.Realtime{}, &core.Storage{}, &core.Studio{}, &core.EdgeRuntime{}}})
}
