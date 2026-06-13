/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package singledatabase

import (
	"fmt"
	"maps"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceName returns the name of the Service for a SingleDatabase.
func ServiceName(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s-db", db.Name)
}

// BuildService constructs the Service for a SingleDatabase.
func BuildService(db *supabasev1alpha1.SingleDatabase) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceName(db),
			Namespace:   db.Namespace,
			Labels:      ServiceLabels(db),
			Annotations: ServiceAnnotations(db),
		},
		Spec: corev1.ServiceSpec{
			Type:     ServiceType(db),
			Selector: DefaultLabels(db.Name),
			Ports: []corev1.ServicePort{
				{
					Name:       DefaultContainerPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       DefaultPort,
					TargetPort: intstr.FromInt32(DefaultPort),
				},
			},
		},
	}
}

// ServiceType returns the Service type, defaulting when unset.
func ServiceType(db *supabasev1alpha1.SingleDatabase) corev1.ServiceType {
	if db.Spec.Service != nil && db.Spec.Service.Type != nil && *db.Spec.Service.Type != "" {
		return *db.Spec.Service.Type
	}
	return DefaultServiceType
}

// ServiceLabels returns default labels merged with user-provided service labels.
func ServiceLabels(db *supabasev1alpha1.SingleDatabase) map[string]string {
	labels := DefaultLabels(db.Name)
	if db.Spec.Service != nil {
		maps.Copy(labels, db.Spec.Service.Labels)
	}
	return labels
}

// ServiceAnnotations returns annotations merged with user-provided service annotations.
func ServiceAnnotations(db *supabasev1alpha1.SingleDatabase) map[string]string {
	annotations := map[string]string{}
	if db.Spec.Service != nil {
		maps.Copy(annotations, db.Spec.Service.Annotations)
	}
	return annotations
}
