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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/supabase-community/supabase-kubernetes/internal/helper"
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
			Name:      FunctionsServiceName(project),
			Namespace: project.Namespace,
			Labels:    FunctionsLabels(project),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
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

	return helper.OverlayService(*svc, project.Spec.EdgeRuntime.Service)
}
