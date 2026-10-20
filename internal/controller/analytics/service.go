package analytics

import (
	"context"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	svc, err := analyticsdefaults.AnalyticsService(state)
	if err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, svc, state.Owner, reconciler.MutateService(), "Analytics Service")
}
