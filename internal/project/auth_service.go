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

// AuthServiceName returns the name of the Auth Service for a Project.
func AuthServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-auth", project.Name)
}

// AuthService constructs the Auth Service for a Project.
func AuthService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Auth == nil || !*project.Spec.Auth.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        AuthServiceName(project),
			Namespace:   project.Namespace,
			Labels:      authServiceLabels(project),
			Annotations: authServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:     authServiceType(project),
			Selector: AuthSelectorLabels(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "auth",
					Port:       DefaultAuthPort,
					TargetPort: intstr.FromInt32(DefaultAuthPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// authServiceLabels returns the merged Service labels for the Auth component.
func authServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(AuthLabels(project))
	if project.Spec.Auth != nil && project.Spec.Auth.Service != nil {
		maps.Copy(labels, project.Spec.Auth.Service.Labels)
	}
	return labels
}

// authServiceAnnotations returns the Service annotations for the Auth component.
func authServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Auth == nil || project.Spec.Auth.Service == nil {
		return nil
	}
	return project.Spec.Auth.Service.Annotations
}

// authServiceType returns the service type from the spec or ClusterIP.
func authServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Auth != nil && project.Spec.Auth.Service != nil && project.Spec.Auth.Service.Type != nil {
		return *project.Spec.Auth.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}
