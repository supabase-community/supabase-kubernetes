package analytics

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileSecret(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	passwordSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: state.Namespace, Name: db.PasswordRef.Name}, passwordSecret); err != nil {
		return fmt.Errorf("reading database password secret: %w", err)
	}
	password, ok := passwordSecret.Data[db.PasswordRef.Key]
	if !ok {
		return fmt.Errorf("database password secret %s is missing key %s", db.PasswordRef.Name, db.PasswordRef.Key)
	}
	desired, err := analyticsdefaults.AnalyticsAuthSecret(state, db, string(password))
	if err != nil {
		return fmt.Errorf("building analytics auth secret: %w", err)
	}
	preserveTokens := reconciler.MutateSecret(analyticsdefaults.AuthSecretPublicTokenKey, analyticsdefaults.AuthSecretPrivateTokenKey)
	return component.Ensure(ctx, r.Client, desired, state.Owner, func(existing, desired *corev1.Secret) error {
		if existing.Data == nil {
			existing.Data = map[string][]byte{}
		}
		if err := preserveTokens(existing, desired); err != nil {
			return err
		}
		existing.Data[analyticsdefaults.AuthSecretURLKey] = desired.Data[analyticsdefaults.AuthSecretURLKey]
		return nil
	}, "Analytics Secret")
}
