package studio

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	studiodefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/studio"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileStatefulSet(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	workload, err := studiodefaults.StudioStatefulSet(state, db)
	if err != nil {
		return fmt.Errorf("building studio statefulset: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &workload.Spec.Template); err != nil {
		return err
	}
	if err := component.EnsureFunctionSyncAccess(ctx, r.Client, state.Owner, workload.Name, &workload.Spec.Template.Spec); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, workload, state.Owner, reconciler.MutateStatefulSet(), "Studio StatefulSet")
}
