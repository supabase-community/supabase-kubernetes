package envoy

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	envoydefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/envoy"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

func (r *Reconciler) reconcileDeployment(ctx context.Context, state *defaults.Context, configHash, secretHash string) error {
	deployment, err := envoydefaults.EnvoyDeployment(state, configHash, secretHash)
	if err != nil {
		return fmt.Errorf("building envoy deployment: %w", err)
	}
	if err := component.StampInputs(ctx, r.Client, state.Namespace, &deployment.Spec.Template); err != nil {
		return err
	}
	return component.Ensure(ctx, r.Client, deployment, state.Owner, reconciler.MutateDeployment(), "Envoy Deployment")
}
