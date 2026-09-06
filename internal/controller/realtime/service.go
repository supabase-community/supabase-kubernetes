package realtime

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	realtimedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/realtime"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := realtimedefaults.RealtimeService(state)
	if err != nil {
		return fmt.Errorf("building realtime service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Realtime Service")
}
