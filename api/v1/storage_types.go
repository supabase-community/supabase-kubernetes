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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageSpec defines the desired state of Storage.
type StorageSpec struct {
	// ProjectRef references the Project resource that owns this Storage component.
	// +kubebuilder:validation:Required
	ProjectRef corev1.LocalObjectReference `json:"projectRef"`

	// Replicas defines the number of Storage instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the Storage pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the Storage service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines Storage-specific configuration
	// +optional
	Config StorageConfig `json:"config,omitempty"`

	// Storage defines the persistent volume claims for Storage data
	// +optional
	Storage []corev1.PersistentVolumeClaim `json:"storage,omitempty"`
}

// StorageConfig defines Storage-specific configuration.
type StorageConfig struct {
	// FileSizeLimit defines the maximum file size in bytes
	// +optional
	// +kubebuilder:default=52428800
	// +kubebuilder:validation:Minimum=1
	FileSizeLimit *int64 `json:"fileSizeLimit,omitempty"`
}

// StorageStatus defines the observed state of Storage.
type StorageStatus struct {
	// Conditions represent the latest available observations of the Storage's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=storages,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Storage is the Schema for the storages API.
type Storage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              StorageSpec   `json:"spec"`
	Status            StorageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StorageList contains a list of Storage.
type StorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Storage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Storage{}, &StorageList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (s *Storage) GetConditions() *[]metav1.Condition {
	return &s.Status.Conditions
}
