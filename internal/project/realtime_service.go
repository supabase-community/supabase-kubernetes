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

// RealtimeServiceName returns the name of the Realtime Service for a Project.
func RealtimeServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-realtime", project.Name)
}

// RealtimeService constructs the Realtime Service for a Project.
func RealtimeService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Realtime == nil || !*project.Spec.Realtime.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RealtimeServiceName(project),
			Namespace:   project.Namespace,
			Labels:      realtimeServiceLabels(project),
			Annotations: realtimeServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:           realtimeServiceType(project),
			Selector:       RealtimeSelectorLabels(project),
			IPFamilies:     realtimeServiceIPFamilies(project),
			IPFamilyPolicy: realtimeServiceIPFamilyPolicy(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "realtime",
					Port:       DefaultRealtimePort,
					TargetPort: intstr.FromInt32(DefaultRealtimePort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc, nil
}

// realtimeServiceLabels returns the merged Service labels for the Realtime component.
func realtimeServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(RealtimeLabels(project))
	if project.Spec.Realtime != nil && project.Spec.Realtime.Service != nil {
		maps.Copy(labels, project.Spec.Realtime.Service.Labels)
	}
	return labels
}

// realtimeServiceAnnotations returns the Service annotations for the Realtime component.
func realtimeServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Realtime == nil || project.Spec.Realtime.Service == nil {
		return nil
	}
	return project.Spec.Realtime.Service.Annotations
}

// realtimeServiceType returns the service type from the spec or ClusterIP.
func realtimeServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Realtime != nil && project.Spec.Realtime.Service != nil && project.Spec.Realtime.Service.Type != nil {
		return *project.Spec.Realtime.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// realtimeServiceIPFamilyPolicy returns the service IPFamilyPolicy from the spec or ClusterIP.
func realtimeServiceIPFamilyPolicy(project *supabasev1alpha1.Project) *corev1.IPFamilyPolicy {
	if project.Spec.Realtime != nil && project.Spec.Realtime.Service != nil && project.Spec.Realtime.Service.IPFamilyPolicy != nil {
		return project.Spec.Realtime.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}

// realtimeServiceIPFamilies returns the service IPFamilyPolies from the spec.
func realtimeServiceIPFamilies(project *supabasev1alpha1.Project) []corev1.IPFamily {
	if project.Spec.Realtime != nil && project.Spec.Realtime.Service != nil && project.Spec.Realtime.Service.IPFamilies != nil {
		return project.Spec.Realtime.Service.IPFamilies
	}
	defaultPolicy := []corev1.IPFamily{corev1.IPv4Protocol}
	return defaultPolicy
}
