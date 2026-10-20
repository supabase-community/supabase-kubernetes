package vector

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VectorServiceAccount constructs the identity used by the log collector.
func VectorServiceAccount(project *ResourceContext) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VectorName(project),
			Namespace: project.Namespace,
			Labels:    VectorLabels(project),
		},
	}
}
