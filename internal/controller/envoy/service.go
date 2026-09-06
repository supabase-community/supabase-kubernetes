package envoy

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	envoydefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/envoy"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := envoydefaults.EnvoyService(state)
	if err != nil {
		return fmt.Errorf("building envoy service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Envoy Service")
}
