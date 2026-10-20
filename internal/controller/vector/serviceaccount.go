package vector

import (
	"context"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	vectordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/vector"
	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) reconcileServiceAccount(ctx context.Context, state *defaults.Context) error {
	desired := vectordefaults.VectorServiceAccount(state)
	return component.Ensure(ctx, r.Client, desired, state.Owner, func(existing, desired *corev1.ServiceAccount) error {
		existing.Labels = desired.Labels
		return nil
	}, "Vector ServiceAccount")
}
