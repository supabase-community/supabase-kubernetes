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

package meta

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MetaServiceName returns the name of the Meta Service for a Project.
func MetaServiceName(project *ResourceContext) string {
	return ComponentName(project.Names["Meta"], "meta")
}

// MetaService constructs the Meta Service for a Project.
func MetaService(project *ResourceContext) (*corev1.Service, error) {
	if project.Spec.Meta == nil {
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
			Type:     metaServiceType(project),
			Selector: MetaSelectorLabels(project),
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
func metaServiceLabels(project *ResourceContext) map[string]string {
	labels := maps.Clone(MetaLabels(project))
	if project.Spec.Meta != nil && project.Spec.Meta.Service != nil {
		maps.Copy(labels, project.Spec.Meta.Service.Labels)
	}
	return labels
}

// metaServiceAnnotations returns the Service annotations for the Meta component.
func metaServiceAnnotations(project *ResourceContext) map[string]string {
	if project.Spec.Meta == nil || project.Spec.Meta.Service == nil {
		return nil
	}
	return project.Spec.Meta.Service.Annotations
}

// metaServiceType returns the service type from the spec or ClusterIP.
func metaServiceType(project *ResourceContext) corev1.ServiceType {
	if project.Spec.Meta != nil && project.Spec.Meta.Service != nil && project.Spec.Meta.Service.Type != nil {
		return *project.Spec.Meta.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}
