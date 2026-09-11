package edgeruntime

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	edgedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileDeployment(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	deployment, err := edgedefaults.FunctionsDeployment(state, db)
	if err != nil {
		return fmt.Errorf("building functions deployment: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &deployment.Spec.Template); err != nil {
		return err
	}
	if err := component.EnsureFunctionSyncAccess(ctx, r.Client, state.Owner, deployment.Name, &deployment.Spec.Template.Spec); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, deployment, state.Owner, reconciler.MutateDeployment(), "Functions Deployment")
}
