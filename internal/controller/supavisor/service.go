package supavisor

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	supavisordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/supavisor"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := supavisordefaults.SupavisorService(state)
	if err != nil {
		return fmt.Errorf("building supavisor service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Supavisor Service")
}
