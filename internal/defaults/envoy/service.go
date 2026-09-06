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

package envoy

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// EnvoyServiceName returns the name of the Envoy Service for a Project.
func EnvoyServiceName(project *ResourceContext) string {
	return ComponentName(project.Names["Envoy"], "envoy")
}

// EnvoyService constructs the Envoy Service for a Project.
func EnvoyService(project *ResourceContext) (*corev1.Service, error) {
	if project.Spec.Envoy == nil {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EnvoyServiceName(project),
			Namespace: project.Namespace,
			Labels:    EnvoyLabels(project),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: EnvoySelectorLabels(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "envoy",
					Port:       DefaultEnvoyPort,
					TargetPort: intstr.FromInt32(DefaultEnvoyPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return helper.OverlayService(*svc, project.Spec.Envoy.Service)
}
