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

package storage

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// StorageServiceName returns the name of the Storage Service for a Project.
func StorageServiceName(project *ResourceContext) string {
	return ComponentName(project.Names["Storage"], "storage")
}

// StorageService constructs the Service for the Storage component.
func StorageService(project *ResourceContext) (*corev1.Service, error) {
	if project.Spec.Storage == nil {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StorageServiceName(project),
			Namespace: project.Namespace,
			Labels:    StorageLabels(project),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: StorageSelectorLabels(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "storage",
					Port:       DefaultStoragePort,
					TargetPort: intstr.FromInt32(DefaultStoragePort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return helper.OverlayService(*svc, project.Spec.Storage.Service)
}
