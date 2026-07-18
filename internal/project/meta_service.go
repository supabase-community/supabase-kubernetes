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

// MetaServiceName returns the name of the Meta Service for a Project.
func MetaServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-meta", project.Name)
}

// MetaService constructs the Meta Service for a Project.
func MetaService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Meta == nil || !*project.Spec.Meta.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        MetaServiceName(project),
			Namespace:   project.Namespace,
			Labels:      metaServiceLabels(project),
			Annotations: metaServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:           metaServiceType(project),
			Selector:       MetaSelectorLabels(project),
			IPFamilies:     metaServiceIPFamilies(project),
			IPFamilyPolicy: metaServiceIPFamilyPolicy(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "meta",
					Port:       DefaultMetaPort,
					TargetPort: intstr.FromInt32(DefaultMetaPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// metaServiceLabels returns the merged Service labels for the Meta component.
func metaServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(MetaLabels(project))
	if project.Spec.Meta != nil && project.Spec.Meta.Service != nil {
		maps.Copy(labels, project.Spec.Meta.Service.Labels)
	}
	return labels
}

// metaServiceAnnotations returns the Service annotations for the Meta component.
func metaServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Meta == nil || project.Spec.Meta.Service == nil {
		return nil
	}
	return project.Spec.Meta.Service.Annotations
}

// metaServiceType returns the service type from the spec or ClusterIP.
func metaServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Meta != nil && project.Spec.Meta.Service != nil && project.Spec.Meta.Service.Type != nil {
		return *project.Spec.Meta.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// metaServiceIPFamilyPolicy returns the service IPFamilyPolicy from the spec or ClusterIP.
func metaServiceIPFamilyPolicy(project *supabasev1alpha1.Project) *corev1.IPFamilyPolicy {
	if project.Spec.Meta != nil && project.Spec.Meta.Service != nil && project.Spec.Meta.Service.IPFamilyPolicy != nil {
		return project.Spec.Meta.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}

// metaServiceIPFamilies returns the service IPFamilyPolies from the spec.
func metaServiceIPFamilies(project *supabasev1alpha1.Project) []corev1.IPFamily {
	if project.Spec.Meta != nil && project.Spec.Meta.Service != nil && project.Spec.Meta.Service.IPFamilies != nil {
		return project.Spec.Meta.Service.IPFamilies
	}
	defaultPolicy := []corev1.IPFamily{corev1.IPv4Protocol}
	return defaultPolicy
}
