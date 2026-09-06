package rest

import (
	"context"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	restdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/rest"
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

// +kubebuilder:rbac:groups=core.supabase.io,resources=rests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=rests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=rests/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &core.Rest{}
	project, result, proceed, err := component.Start(ctx, r.Client, req, obj, &core.RestList{}, "Rest")
	if err != nil || !proceed {
		return result, err
	}
	db, result, proceed, err := component.ResolveDatabase(ctx, r.Client, obj, project)
	if err != nil || !proceed {
		return result, err
	}
	state := defaults.NewContext(project)
	state.Owner = obj
	state.Spec.Rest = &obj.Spec
	state.Names["Rest"] = obj.Name
	if err := r.reconcileService(ctx, state); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := r.reconcileDeployment(ctx, state, db); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := component.DeploymentReady(ctx, r.Client, client.ObjectKey{Namespace: obj.Namespace, Name: restdefaults.RestDeploymentName(state)}); err != nil {
		return component.NotReady(ctx, r.Client, obj, "WorkloadNotReady", err.Error())
	}
	return component.Ready(ctx, r.Client, obj)
}
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return component.SetupWithManager(mgr, r, component.SetupOptions{Object: &core.Rest{}, NewList: func() client.ObjectList { return &core.RestList{} }, Owns: []client.Object{&appsv1.Deployment{}, &corev1.Service{}}})
}
