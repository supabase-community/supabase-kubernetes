package edgeruntime

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	edgedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileMainFunction(ctx context.Context, state *defaults.Context) error {
	function, err := edgedefaults.ProjectMainFunction(state)
	if err != nil {
		return fmt.Errorf("building main function: %w", err)
	}
	return component.Ensure(ctx, r.Client, function, state.Owner, reconciler.MutateFunction(), "main Function")
}
