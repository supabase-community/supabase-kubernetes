package envoy

import (
	"context"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	envoydefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/envoy"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileSecret(ctx context.Context, state *defaults.Context) (string, error) {
	desired, err := envoydefaults.EnvoySecret(state)
	if err != nil {
		return "", fmt.Errorf("building envoy secret: %w", err)
	}
	if err := component.Ensure(ctx, r.Client, desired, state.Owner, reconciler.MutateSecret(envoydefaults.DefaultEnvoySecretKeyUsername, envoydefaults.DefaultEnvoySecretKeyPassword), "Envoy Secret"); err != nil {
		return "", err
	}
	actual := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(desired), actual); err != nil {
		return "", err
	}
	return helper.SecretHash(actual), nil
}
