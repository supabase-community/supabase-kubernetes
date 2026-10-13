package edgeruntime

import (
	"context"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	edgedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileConfigMap(ctx context.Context, state *defaults.Context) error {
	config := edgedefaults.RuntimeConfigMap(state)
	return component.Ensure(ctx, r.Client, config, state.Owner, reconciler.MutateConfigMap(), "EdgeRuntime ConfigMap")
}
