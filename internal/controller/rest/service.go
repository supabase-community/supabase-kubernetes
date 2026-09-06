package rest

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	restdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/rest"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := restdefaults.RestService(state)
	if err != nil {
		return fmt.Errorf("building rest service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Rest Service")
}
