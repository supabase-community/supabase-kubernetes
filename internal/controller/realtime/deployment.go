package realtime

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	realtimedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/realtime"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileDeployment(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	deployment, err := realtimedefaults.RealtimeDeployment(state, db)
	if err != nil {
		return fmt.Errorf("building realtime deployment: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &deployment.Spec.Template); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, deployment, state.Owner, reconciler.MutateDeployment(), "Realtime Deployment")
}
