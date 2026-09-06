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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// PostgresServiceName returns the name of the Service for a SingleDatabase.
func PostgresServiceName(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s-postgres", db.Name)
}

// PostgresService constructs the Service for a SingleDatabase.
func PostgresService(db *supabasev1alpha1.SingleDatabase) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PostgresServiceName(db),
			Namespace: db.Namespace,
			Labels:    PostgresLabels(db),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: PostgresSelectorLabels(db),
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

	return helper.OverlayService(*svc, db.Spec.Service)
}
