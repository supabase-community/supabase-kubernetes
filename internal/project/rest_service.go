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

// RestServiceName returns the name of the Rest Service for a Project.
func RestServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-rest", project.Name)
}

// RestService constructs the Rest Service for a Project.
func RestService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Rest == nil || !*project.Spec.Rest.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RestServiceName(project),
			Namespace:   project.Namespace,
			Labels:      restServiceLabels(project),
			Annotations: restServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:     restServiceType(project),
			Selector: RestSelectorLabels(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "rest",
					Port:       DefaultRestPort,
					TargetPort: intstr.FromInt32(DefaultRestPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// restServiceLabels returns the merged Service labels for the Rest component.
func restServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(RestLabels(project))
	if project.Spec.Rest != nil && project.Spec.Rest.Service != nil {
		maps.Copy(labels, project.Spec.Rest.Service.Labels)
	}
	return labels
}

// restServiceAnnotations returns the Service annotations for the Rest component.
func restServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Rest == nil || project.Spec.Rest.Service == nil {
		return nil
	}
	return project.Spec.Rest.Service.Annotations
}

// restServiceType returns the service type from the spec or ClusterIP.
func restServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Rest != nil && project.Spec.Rest.Service != nil && project.Spec.Rest.Service.Type != nil {
		return *project.Spec.Rest.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}
