package storage

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	storagedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/storage"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileStatefulSet(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	workload, err := storagedefaults.StorageStatefulSet(state, db)
	if err != nil {
		return fmt.Errorf("building storage statefulset: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &workload.Spec.Template); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, workload, state.Owner, reconciler.MutateStatefulSet(), "Storage StatefulSet")
}
