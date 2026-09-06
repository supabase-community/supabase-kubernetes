package storage

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	storagedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/storage"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcilePVC(ctx context.Context, state *defaults.Context) error {
	pvc, err := storagedefaults.StoragePVC(state)
	if err != nil {
		return fmt.Errorf("building storage pvc: %w", err)
	}
	pvc.Annotations = map[string]string{"core.supabase.io/owner-uid": string(state.Owner.GetUID())}
	return component.Ensure(ctx, r.Client, pvc, state.Owner, reconciler.MutatePVC(), "Storage PVC")
}
