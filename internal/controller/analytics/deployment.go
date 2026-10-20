package analytics

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func deleteAnalyticsDeployment(ctx context.Context, c client.Client, obj *core.Analytics) error {
	d := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: obj.Namespace, Name: defaults.ComponentName(obj.Name, "analytics")}
	if err := c.Get(ctx, key, d); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(d, obj) {
		return fmt.Errorf("deployment %s belongs to another owner", d.Name)
	}
	return client.IgnoreNotFound(c.Delete(ctx, d))
}

func (r *Reconciler) reconcileDeployment(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	d, err := analyticsdefaults.AnalyticsDeployment(state, db)
	if err != nil {
		return err
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &d.Spec.Template); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, d, state.Owner, reconciler.MutateDeployment(), "Analytics Deployment")
}
