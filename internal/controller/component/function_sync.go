package component

import (
	"context"

	"github.com/supabase-community/supabase-kubernetes/internal/defaults/function"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

// EnsureFunctionSyncAccess binds read access to the account selected after overlay.
// Accounts supplied by the user are referenced, never adopted or modified.
func EnsureFunctionSyncAccess(ctx context.Context, c client.Client, owner client.Object, workloadName string, pod *corev1.PodSpec) error {
	name := function.SyncServiceAccountName(workloadName)
	metadata := metav1.ObjectMeta{Name: name, Namespace: owner.GetNamespace()}
	account := pod.ServiceAccountName
	if account == "" {
		account = "default"
	}
	if account == name {
		if err := Ensure(ctx, c, &corev1.ServiceAccount{ObjectMeta: metadata}, owner, func(existing, desired *corev1.ServiceAccount) error {
			existing.OwnerReferences = desired.OwnerReferences
			return nil
		}, "Function sync ServiceAccount"); err != nil {
			return err
		}
	}
	role := &rbacv1.Role{ObjectMeta: metadata, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"core.supabase.io"}, Resources: []string{"functions"}, Verbs: []string{"list"}}}}
	if err := Ensure(ctx, c, role, owner, func(existing, desired *rbacv1.Role) error {
		existing.Rules = desired.Rules
		existing.OwnerReferences = desired.OwnerReferences
		return nil
	}, "Function sync Role"); err != nil {
		return err
	}
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metadata,
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: account, Namespace: owner.GetNamespace()}},
	}
	return Ensure(ctx, c, binding, owner, func(existing, desired *rbacv1.RoleBinding) error {
		existing.Subjects = desired.Subjects
		existing.RoleRef = desired.RoleRef
		existing.OwnerReferences = desired.OwnerReferences
		return nil
	}, "Function sync RoleBinding")
}
