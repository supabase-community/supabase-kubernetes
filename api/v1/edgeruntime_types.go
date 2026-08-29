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

// EdgeRuntimeSpec defines the desired state of EdgeRuntime.
type EdgeRuntimeSpec struct {
	// ProjectRef references the Project resource that owns this EdgeRuntime component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProjectRef string `json:"projectRef"`

	// Replicas defines the number of EdgeRuntime instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the EdgeRuntime pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the EdgeRuntime service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines EdgeRuntime-specific configuration
	// +optional
	Config EdgeRuntimeConfig `json:"config,omitempty"`
}

// EdgeRuntimeConfig defines EdgeRuntime-specific configuration.
type EdgeRuntimeConfig struct {
	// VerifyJWT defines whether to verify JWT tokens
	// +optional
	// +kubebuilder:default=true
	VerifyJWT *bool `json:"verifyJwt,omitempty"`
}

// EdgeRuntimeStatus defines the observed state of EdgeRuntime.
type EdgeRuntimeStatus struct {
	// Conditions represent the latest available observations of the EdgeRuntime's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=edgeruntimes,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// EdgeRuntime is the Schema for the edgeruntimes API.
type EdgeRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EdgeRuntimeSpec   `json:"spec"`
	Status            EdgeRuntimeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EdgeRuntimeList contains a list of EdgeRuntime.
type EdgeRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeRuntime `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeRuntime{}, &EdgeRuntimeList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (e *EdgeRuntime) GetConditions() *[]metav1.Condition {
	return &e.Status.Conditions
}
