package vector

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	vectordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/vector"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) removeDaemonSet(ctx context.Context, obj *core.Vector) error {
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: defaults.ComponentName(obj.Name, "vector")}, ds); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(ds, obj) {
		return fmt.Errorf("DaemonSet %s belongs to another owner", ds.Name)
	}
	return client.IgnoreNotFound(r.Delete(ctx, ds))
}

func (r *Reconciler) reconcileDaemonSet(ctx context.Context, state *defaults.Context) error {
	desired, err := vectordefaults.VectorDaemonSet(state)
	if err != nil {
		return fmt.Errorf("building vector daemonset: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &desired.Spec.Template); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, desired, state.Owner, reconciler.MutateDaemonSet(), "Vector DaemonSet")
}
