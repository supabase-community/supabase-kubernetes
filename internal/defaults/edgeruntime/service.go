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

package edgeruntime

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// FunctionsServiceName returns the name of the Functions Service for a Project.
func FunctionsServiceName(project *ResourceContext) string {
	return ComponentName(project.Names["EdgeRuntime"], "edge-runtime")
}

// FunctionsService constructs the Functions Service for a Project.
func FunctionsService(project *ResourceContext) (*corev1.Service, error) {
	if project.Spec.EdgeRuntime == nil {
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
			Type:     functionsServiceType(project),
			Selector: FunctionsSelectorLabels(project),
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
func functionsServiceLabels(project *ResourceContext) map[string]string {
	labels := maps.Clone(FunctionsLabels(project))
	if project.Spec.EdgeRuntime != nil && project.Spec.EdgeRuntime.Service != nil {
		maps.Copy(labels, project.Spec.EdgeRuntime.Service.Labels)
	}
	return labels
}

// functionsServiceAnnotations returns the Service annotations for the Functions component.
func functionsServiceAnnotations(project *ResourceContext) map[string]string {
	if project.Spec.EdgeRuntime == nil || project.Spec.EdgeRuntime.Service == nil {
		return nil
	}
	return project.Spec.EdgeRuntime.Service.Annotations
}

// functionsServiceType returns the service type from the spec or ClusterIP.
func functionsServiceType(project *ResourceContext) corev1.ServiceType {
	if project.Spec.EdgeRuntime != nil && project.Spec.EdgeRuntime.Service != nil && project.Spec.EdgeRuntime.Service.Type != nil {
		return *project.Spec.EdgeRuntime.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}
