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

// MetaSpec defines the desired state of Meta.
type MetaSpec struct {
	// ProjectRef references the Project resource that owns this Meta component.
	// +kubebuilder:validation:Required
	ProjectRef corev1.LocalObjectReference `json:"projectRef"`

	// Replicas defines the number of Meta instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the Meta pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the Meta service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines Meta-specific configuration
	// +optional
	Config MetaConfig `json:"config,omitempty"`
}

// MetaConfig defines Meta-specific configuration.
type MetaConfig struct {
}

// MetaStatus defines the observed state of Meta.
type MetaStatus struct {
	// Conditions represent the latest available observations of the Meta's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=metas,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Meta is the Schema for the metas API.
type Meta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MetaSpec   `json:"spec"`
	Status            MetaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MetaList contains a list of Meta.
type MetaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Meta `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Meta{}, &MetaList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (m *Meta) GetConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}
