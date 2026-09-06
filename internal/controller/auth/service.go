package auth

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	authdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/auth"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := authdefaults.AuthService(state)
	if err != nil {
		return fmt.Errorf("building auth service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Auth Service")
}
