package supavisor

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	supavisordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/supavisor"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
)

func (r *Reconciler) reconcileDeployment(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	deployment, err := supavisordefaults.SupavisorDeployment(state, db)
	if err != nil {
		return fmt.Errorf("building supavisor deployment: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &deployment.Spec.Template); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, deployment, state.Owner, func(existing, desired *appsv1.Deployment) error {
		if err := reconciler.MutateDeployment()(existing, desired); err != nil {
			return err
		}
		existing.Spec.Strategy = desired.Spec.Strategy
		return nil
	}, "Supavisor Deployment")
}
