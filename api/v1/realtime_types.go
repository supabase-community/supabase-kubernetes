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

// RealtimeSpec defines the desired state of Realtime.
type RealtimeSpec struct {
	// ProjectRef references the Project resource that owns this Realtime component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProjectRef string `json:"projectRef"`

	// Replicas defines the number of Realtime instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the Realtime pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the Realtime service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines Realtime-specific configuration
	// +optional
	Config RealtimeConfig `json:"config,omitempty"`
}

// RealtimeConfig defines Realtime-specific configuration.
type RealtimeConfig struct {
}

// RealtimeStatus defines the observed state of Realtime.
type RealtimeStatus struct {
	// Conditions represent the latest available observations of the Realtime's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=realtime,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Realtime is the Schema for the realtime API.
type Realtime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RealtimeSpec   `json:"spec"`
	Status            RealtimeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RealtimeList contains a list of Realtime.
type RealtimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Realtime `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Realtime{}, &RealtimeList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (r *Realtime) GetConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
