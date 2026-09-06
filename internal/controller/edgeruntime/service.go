package edgeruntime

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	edgedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := edgedefaults.FunctionsService(state)
	if err != nil {
		return fmt.Errorf("building functions service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Functions Service")
}
