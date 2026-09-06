package studio

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	studiodefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/studio"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := studiodefaults.StudioService(state)
	if err != nil {
		return fmt.Errorf("building studio service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Studio Service")
}
