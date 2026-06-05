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

// ServiceName returns the name of the Service for a SingleDatabase.
func ServiceName(dbName string) string {
	return fmt.Sprintf("%s-db", dbName)
}

// BuildService constructs the Service for a SingleDatabase.
func BuildService(db *supabasev1alpha1.SingleDatabase) *corev1.Service {
	labels := DefaultLabels(db.Name)
	annotations := map[string]string{}
	svcType := corev1.ServiceTypeClusterIP
	if db.Spec.Service != nil {
		if db.Spec.Service.Type != "" {
			svcType = db.Spec.Service.Type
		}
		maps.Copy(annotations, db.Spec.Service.Annotations)
		maps.Copy(labels, db.Spec.Service.Labels)
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceName(db.Name),
			Namespace:   db.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: svcType,
			Selector: map[string]string{
				"app.kubernetes.io/name":      AppName,
				"app.kubernetes.io/instance":  db.Name,
				"app.kubernetes.io/component": Component,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Protocol:   corev1.ProtocolTCP,
					Port:       Port,
					TargetPort: intstr.FromInt32(Port),
				},
			},
		},
	}
}
