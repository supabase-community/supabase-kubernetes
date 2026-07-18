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

package project

import (
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// EnvoyServiceName returns the name of the Envoy Service for a Project.
func EnvoyServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-envoy", project.Name)
}

// EnvoyService constructs the Envoy Service for a Project.
func EnvoyService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Envoy == nil || !*project.Spec.Envoy.Enable {
		return nil, nil
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        EnvoyServiceName(project),
			Namespace:   project.Namespace,
			Labels:      envoyServiceLabels(project),
			Annotations: envoyServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:           envoyServiceType(project),
			Selector:       EnvoySelectorLabels(project),
			IPFamilies:     envoyServiceIPFamilies(project),
			IPFamilyPolicy: envoyServiceIPFamilyPolicy(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "envoy",
					Port:       DefaultEnvoyPort,
					TargetPort: intstr.FromInt32(DefaultEnvoyPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}, nil
}

// envoyServiceLabels returns the merged Service labels for the Envoy component.
func envoyServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(EnvoyLabels(project))
	if project.Spec.Envoy != nil && project.Spec.Envoy.Service != nil {
		maps.Copy(labels, project.Spec.Envoy.Service.Labels)
	}
	return labels
}

// envoyServiceAnnotations returns the Service annotations for the Envoy component.
func envoyServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Envoy == nil || project.Spec.Envoy.Service == nil {
		return nil
	}
	return project.Spec.Envoy.Service.Annotations
}

// envoyServiceType returns the service type from the spec or ClusterIP.
func envoyServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Envoy != nil && project.Spec.Envoy.Service != nil && project.Spec.Envoy.Service.Type != nil {
		return *project.Spec.Envoy.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// envoyServiceIPFamilyPolicy returns the service IPFamilyPolicy from the spec.
func envoyServiceIPFamilyPolicy(project *supabasev1alpha1.Project) *corev1.IPFamilyPolicy {
	if project.Spec.Envoy != nil && project.Spec.Envoy.Service != nil && project.Spec.Envoy.Service.IPFamilyPolicy != nil {
		return project.Spec.Envoy.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}

// envoyServiceIPFamilies returns the service IPFamilyPolies from the spec.
func envoyServiceIPFamilies(project *supabasev1alpha1.Project) []corev1.IPFamily {
	if project.Spec.Envoy != nil && project.Spec.Envoy.Service != nil && project.Spec.Envoy.Service.IPFamilies != nil {
		return project.Spec.Envoy.Service.IPFamilies
	}
	defaultPolicy := []corev1.IPFamily{corev1.IPv4Protocol}
	return defaultPolicy
}
