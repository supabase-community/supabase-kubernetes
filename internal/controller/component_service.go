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

package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// ComponentServiceParams holds parameters for building a component Service.
type ComponentServiceParams struct {
	Component string
	Port      int32
}

// BuildComponentService creates a ClusterIP Service for a Supabase component.
func BuildComponentService(project *platformv1alpha1.Project, params ComponentServiceParams) *corev1.Service {
	labels := componentLabels(project, params.Component)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentServiceName(project.Name, params.Component),
			Namespace: project.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       params.Port,
				TargetPort: intstr.FromInt32(params.Port),
			}},
		},
	}
}
