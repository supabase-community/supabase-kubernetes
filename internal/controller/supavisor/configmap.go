package supavisor

import (
	"context"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	supavisordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/supavisor"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileConfigMap(ctx context.Context, state *defaults.Context) error {
	return component.Ensure(ctx, r.Client, supavisordefaults.SupavisorConfigMap(state), state.Owner, reconciler.MutateConfigMap(), "Supavisor ConfigMap")
}
