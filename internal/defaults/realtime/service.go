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

package realtime

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// RealtimeServiceName returns the name of the Realtime Service for a Project.
func RealtimeServiceName(project *ResourceContext) string {
	return ComponentName(project.Names["Realtime"], "realtime")
}

// RealtimeService constructs the Realtime Service for a Project.
func RealtimeService(project *ResourceContext) (*corev1.Service, error) {
	if project.Spec.Realtime == nil {
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
			Type:     realtimeServiceType(project),
			Selector: RealtimeSelectorLabels(project),
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
func realtimeServiceLabels(project *ResourceContext) map[string]string {
	labels := maps.Clone(RealtimeLabels(project))
	if project.Spec.Realtime != nil && project.Spec.Realtime.Service != nil {
		maps.Copy(labels, project.Spec.Realtime.Service.Labels)
	}
	return labels
}

// realtimeServiceAnnotations returns the Service annotations for the Realtime component.
func realtimeServiceAnnotations(project *ResourceContext) map[string]string {
	if project.Spec.Realtime == nil || project.Spec.Realtime.Service == nil {
		return nil
	}
	return project.Spec.Realtime.Service.Annotations
}

// realtimeServiceType returns the service type from the spec or ClusterIP.
func realtimeServiceType(project *ResourceContext) corev1.ServiceType {
	if project.Spec.Realtime != nil && project.Spec.Realtime.Service != nil && project.Spec.Realtime.Service.Type != nil {
		return *project.Spec.Realtime.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}
