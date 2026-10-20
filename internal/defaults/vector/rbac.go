package vector

import (
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const VectorOwnerAnnotation = "core.supabase.io/owner-uid"

// VectorRBACName uniquely identifies cluster-scoped permissions for a Vector CR.
func VectorRBACName(v *core.Vector) string {
	return "supabase-vector-" + string(v.UID)
}

// VectorClusterRole grants read access to Kubernetes log metadata.
func VectorClusterRole(v *core.Vector) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: vectorRBACMetadata(v),
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"pods", "namespaces", "nodes"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
}

// VectorClusterRoleBinding binds metadata permissions to this project's collector.
func VectorClusterRoleBinding(project *ResourceContext, v *core.Vector) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: vectorRBACMetadata(v),
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     VectorRBACName(v),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      VectorName(project),
			Namespace: project.Namespace,
		}},
	}
}

func vectorRBACMetadata(v *core.Vector) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        VectorRBACName(v),
		Annotations: map[string]string{VectorOwnerAnnotation: string(v.UID)},
	}
}
