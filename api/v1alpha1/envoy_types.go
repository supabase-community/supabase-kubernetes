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

// EnvoySpec defines the desired state of Envoy.
type EnvoySpec struct {
	// ProjectRef references the Project resource that owns this Envoy component.
	// +kubebuilder:validation:Required
	ProjectRef corev1.LocalObjectReference `json:"projectRef"`

	// Replicas defines the number of Envoy instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the Envoy pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the Envoy service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines Envoy-specific configuration
	// +optional
	Config []corev1.EnvVar `json:"config,omitempty"`
}

// EnvoyStatus defines the observed state of Envoy.
type EnvoyStatus struct {
	// Conditions represent the latest available observations of the Envoy's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=envoys,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Envoy is the Schema for the envoys API.
type Envoy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EnvoySpec   `json:"spec"`
	Status            EnvoyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvoyList contains a list of Envoy.
type EnvoyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Envoy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Envoy{}, &EnvoyList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (e *Envoy) GetConditions() *[]metav1.Condition {
	return &e.Status.Conditions
}
