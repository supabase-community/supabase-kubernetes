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

package studio

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// StudioServiceName returns the name of the Studio Service for a Project.
func StudioServiceName(project *ResourceContext) string {
	return ComponentName(project.Names["Studio"], "studio")
}

// StudioService constructs the Studio Service for a Project.
func StudioService(project *ResourceContext) (*corev1.Service, error) {
	if project.Spec.Studio == nil {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StudioServiceName(project),
			Namespace: project.Namespace,
			Labels:    StudioLabels(project),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: StudioSelectorLabels(project),
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

	return helper.OverlayService(*svc, project.Spec.Studio.Service)
}
