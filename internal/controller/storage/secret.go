package storage

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	storagedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/storage"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileSecret(ctx context.Context, state *defaults.Context) error {
	secret, err := storagedefaults.StorageSecret(state)
	if err != nil {
		return fmt.Errorf("building storage secret: %w", err)
	}
	return component.Ensure(ctx, r.Client, secret, state.Owner, reconciler.MutateSecret(storagedefaults.StorageSecretAccessKeyID, storagedefaults.StorageSecretAccessKeySecret), "Storage Secret")
}
