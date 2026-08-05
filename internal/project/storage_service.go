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

// StorageServiceName returns the name of the Storage Service for a Project.
func StorageServiceName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-storage", project.Name)
}

// StorageService constructs the Service for the Storage component.
func StorageService(project *supabasev1alpha1.Project) (*corev1.Service, error) {
	if project.Spec.Storage == nil || !*project.Spec.Storage.Enable {
		return nil, nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        StorageServiceName(project),
			Namespace:   project.Namespace,
			Labels:      storageServiceLabels(project),
			Annotations: storageServiceAnnotations(project),
		},
		Spec: corev1.ServiceSpec{
			Type:           storageServiceType(project),
			Selector:       StorageSelectorLabels(project),
			IPFamilies:     storageServiceIPFamilies(project),
			IPFamilyPolicy: storageServiceIPFamilyPolicy(project),
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

	return svc, nil
}

// storageServiceLabels returns the merged Service labels for the Storage component.
func storageServiceLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(StorageLabels(project))
	if project.Spec.Storage != nil && project.Spec.Storage.Service != nil {
		maps.Copy(labels, project.Spec.Storage.Service.Labels)
	}
	return labels
}

// storageServiceAnnotations returns the Service annotations for the Storage component.
func storageServiceAnnotations(project *supabasev1alpha1.Project) map[string]string {
	if project.Spec.Storage == nil || project.Spec.Storage.Service == nil {
		return nil
	}
	return project.Spec.Storage.Service.Annotations
}

// storageServiceType returns the service type from the spec or ClusterIP.
func storageServiceType(project *supabasev1alpha1.Project) corev1.ServiceType {
	if project.Spec.Storage != nil && project.Spec.Storage.Service != nil && project.Spec.Storage.Service.Type != nil {
		return *project.Spec.Storage.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// storageServiceIPFamilyPolicy returns the service IPFamilyPolicy from the spec or ClusterIP.
func storageServiceIPFamilyPolicy(project *supabasev1alpha1.Project) *corev1.IPFamilyPolicy {
	if project.Spec.Storage != nil && project.Spec.Storage.Service != nil && project.Spec.Storage.Service.IPFamilyPolicy != nil {
		return project.Spec.Storage.Service.IPFamilyPolicy
	}
	defaultPolicy := corev1.IPFamilyPolicySingleStack
	return &defaultPolicy
}

// storageServiceIPFamilies returns the service IPFamilyPolies from the spec.
func storageServiceIPFamilies(project *supabasev1alpha1.Project) []corev1.IPFamily {
	if project.Spec.Storage != nil && project.Spec.Storage.Service != nil && project.Spec.Storage.Service.IPFamilies != nil {
		return project.Spec.Storage.Service.IPFamilies
	}
	defaultPolicy := []corev1.IPFamily{corev1.IPv4Protocol}
	return defaultPolicy
}
