package storage

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	storagedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/storage"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileService(ctx context.Context, state *defaults.Context) error {
	service, err := storagedefaults.StorageService(state)
	if err != nil {
		return fmt.Errorf("building storage service: %w", err)
	}
	return component.Ensure(ctx, r.Client, service, state.Owner, reconciler.MutateService(), "Storage Service")
}
