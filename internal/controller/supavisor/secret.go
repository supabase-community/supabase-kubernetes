package supavisor

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	supavisordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/supavisor"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileSecret(ctx context.Context, state *defaults.Context, db *core.ResolvedDatabase) error {
	passwordSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: state.Namespace, Name: db.PasswordRef.Name}, passwordSecret); err != nil {
		return err
	}
	password, ok := passwordSecret.Data[db.PasswordRef.Key]
	if !ok || len(password) == 0 {
		return fmt.Errorf("database password Secret %s must contain a nonempty %s key", db.PasswordRef.Name, db.PasswordRef.Key)
	}
	secret := supavisordefaults.SupavisorSecret(state, db, string(password))
	return component.Ensure(ctx, r.Client, secret, state.Owner, func(existing, desired *corev1.Secret) error {
		// This is derived data, so password changes must replace the previous URL.
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		existing.Type = desired.Type
		return nil
	}, "Supavisor Secret")
}
