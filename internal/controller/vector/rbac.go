package vector

import (
	"context"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	vectordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/vector"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileRBAC(ctx context.Context, state *defaults.Context, v *core.Vector) error {
	role := vectordefaults.VectorClusterRole(v)
	if _, err := reconciler.EnsureResource(ctx, r.Client, role, nil, func(existing, desired *rbacv1.ClusterRole) error {
		existing.Rules = desired.Rules
		return nil
	}); err != nil {
		return err
	}
	binding := vectordefaults.VectorClusterRoleBinding(state, v)
	_, err := reconciler.EnsureResource(ctx, r.Client, binding, nil, func(existing, desired *rbacv1.ClusterRoleBinding) error {
		if existing.RoleRef != desired.RoleRef {
			return fmt.Errorf("vector role reference is immutable")
		}
		existing.Subjects = desired.Subjects
		return nil
	})
	return err
}

func (r *Reconciler) cleanupRBAC(ctx context.Context, v *core.Vector) error {
	for _, obj := range []client.Object{&rbacv1.ClusterRoleBinding{}, &rbacv1.ClusterRole{}} {
		if err := r.Get(ctx, client.ObjectKey{Name: vectordefaults.VectorRBACName(v)}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if obj.GetAnnotations()[vectordefaults.VectorOwnerAnnotation] != string(v.UID) {
			return fmt.Errorf("RBAC resource %s belongs to another owner", obj.GetName())
		}
		if err := r.Delete(ctx, obj, client.Preconditions{UID: ptr.To(obj.GetUID())}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
