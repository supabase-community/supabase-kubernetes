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

// FunctionsServiceName returns the name of the Functions Service for a Project.
func FunctionsServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-functions", project.Name)
}

// FunctionsService constructs the Functions Service for a Project.
func FunctionsService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Functions == nil || !*project.Spec.Functions.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        FunctionsServiceName(project),
			Namespace:   project.Namespace,
			Labels:      functionsServiceLabels(project),
			Annotations: functionsServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:           functionsServiceType(project),
			Selector:       FunctionsSelectorLabels(project),
			IPFamilies:     functionsServiceIPFamilies(project),
			IPFamilyPolicy: functionsServiceIPFamilyPolicy(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "functions",
					Port:       DefaultFunctionsPort,
					TargetPort: intstr.FromInt32(DefaultFunctionsPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// functionsServiceLabels returns the merged Service labels for the Functions component.
func functionsServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(FunctionsLabels(project))
	if project.Spec.Functions != nil && project.Spec.Functions.Service != nil {
		maps.Copy(labels, project.Spec.Functions.Service.Labels)
	}
	return labels
}

// functionsServiceAnnotations returns the Service annotations for the Functions component.
func functionsServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Functions == nil || project.Spec.Functions.Service == nil {
		return nil
	}
	return project.Spec.Functions.Service.Annotations
}

// functionsServiceType returns the service type from the spec or ClusterIP.
func functionsServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Functions != nil && project.Spec.Functions.Service != nil && project.Spec.Functions.Service.Type != nil {
		return *project.Spec.Functions.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// functionsServiceIPFamilyPolicy returns the service IPFamilyPolicy from the spec or ClusterIP.
func functionsServiceIPFamilyPolicy(project *supabasev1alpha1.Project) *corev1.IPFamilyPolicy {
	if project.Spec.Functions != nil && project.Spec.Functions.Service != nil && project.Spec.Functions.Service.IPFamilyPolicy != nil {
		return project.Spec.Functions.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}

// functionsServiceIPFamilies returns the service IPFamilyPolies from the spec.
func functionsServiceIPFamilies(project *supabasev1alpha1.Project) []corev1.IPFamily {
	if project.Spec.Functions != nil && project.Spec.Functions.Service != nil && project.Spec.Functions.Service.IPFamilies != nil {
		return project.Spec.Functions.Service.IPFamilies
	}
	defaultPolicy := []corev1.IPFamily{corev1.IPv4Protocol}
	return defaultPolicy
}
