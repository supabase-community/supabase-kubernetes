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

// StudioServiceName returns the name of the Studio Service for a Project.
func StudioServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-studio", project.Name)
}

// StudioService constructs the Studio Service for a Project.
func StudioService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Studio == nil || !*project.Spec.Studio.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        StudioServiceName(project),
			Namespace:   project.Namespace,
			Labels:      studioServiceLabels(project),
			Annotations: studioServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:           studioServiceType(project),
			Selector:       StudioSelectorLabels(project),
			IPFamilyPolicy: studioServiceIPFamilyPolicy(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "studio",
					Port:       DefaultStudioPort,
					TargetPort: intstr.FromInt32(DefaultStudioPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// studioServiceLabels returns the merged Service labels for the Studio component.
func studioServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(StudioLabels(project))
	if project.Spec.Studio != nil && project.Spec.Studio.Service != nil {
		maps.Copy(labels, project.Spec.Studio.Service.Labels)
	}
	return labels
}

// studioServiceAnnotations returns the Service annotations for the Studio component.
func studioServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Studio == nil || project.Spec.Studio.Service == nil {
		return nil
	}
	return project.Spec.Studio.Service.Annotations
}

// studioServiceType returns the service type from the spec or ClusterIP.
func studioServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Studio != nil && project.Spec.Studio.Service != nil && project.Spec.Studio.Service.Type != nil {
		return *project.Spec.Studio.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// studioServiceIPFamilyPolicy returns the service IPFamilyPolicy from the spec or ClusterIP.
func studioServiceIPFamilyPolicy(project *supabasev1alpha1.Project) *corev1.IPFamilyPolicy {
	if project.Spec.Studio != nil && project.Spec.Studio.Service != nil && project.Spec.Studio.Service.IPFamilyPolicy != nil {
		return project.Spec.Studio.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}
