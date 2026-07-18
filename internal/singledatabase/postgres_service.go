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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// PostgresServiceName returns the name of the Service for a SingleDatabase.
func PostgresServiceName(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s-postgres", db.Name)
}

// PostgresService constructs the Service for a SingleDatabase.
func PostgresService(db *supabasev1alpha1.SingleDatabase) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        PostgresServiceName(db),
			Namespace:   db.Namespace,
			Labels:      serviceLabels(db),
			Annotations: serviceAnnotations(db),
		},
		Spec: corev1.ServiceSpec{
			Type:           getServiceTypeOrDefault(db),
			Selector:       PostgresSelectorLabels(db),
			IPFamilyPolicy: serviceIPFamilyPolicy(db),
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Port:       DefaultPostgresPort,
					TargetPort: intstr.FromInt32(DefaultPostgresPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// serviceLabels returns the merged Service labels for a SingleDatabase.
func serviceLabels(db *supabasev1alpha1.SingleDatabase) map[string]string {
	labels := maps.Clone(PostgresLabels(db))
	if db.Spec.Service != nil {
		maps.Copy(labels, db.Spec.Service.Labels)
	}
	return labels
}

// serviceAnnotations returns the Service annotations for a SingleDatabase.
func serviceAnnotations(db *supabasev1alpha1.SingleDatabase) map[string]string {
	if db.Spec.Service == nil {
		return nil
	}
	return db.Spec.Service.Annotations
}

// getServiceTypeOrDefault returns the service type from the spec or ClusterIP.
func getServiceTypeOrDefault(db *supabasev1alpha1.SingleDatabase) corev1.ServiceType {
	if db.Spec.Service != nil && db.Spec.Service.Type != nil {
		return *db.Spec.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// serviceIPFamilyPolicy returns the service IPFamilyPolicy from the spec or ClusterIP.
func serviceIPFamilyPolicy(db *supabasev1alpha1.SingleDatabase) *corev1.IPFamilyPolicy {
	if db.Spec.Service != nil && db.Spec.Service.IPFamilyPolicy != nil {
		return db.Spec.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}
