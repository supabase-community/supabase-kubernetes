package vector

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	vectordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/vector"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileConfigMap(ctx context.Context, state *defaults.Context, targets []vectordefaults.Target) error {
	desired, err := vectordefaults.VectorConfigMap(state, targets)
	if err != nil {
		return fmt.Errorf("building vector configmap: %w", err)
	}
	return component.Ensure(ctx, r.Client, desired, state.Owner, reconciler.MutateConfigMap(), "Vector ConfigMap")
}
