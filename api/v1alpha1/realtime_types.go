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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RealtimeSpec defines the desired state of Realtime
type RealtimeSpec struct {
	WorkloadConfig `json:",inline"`
}

// RealtimeStatus defines the observed state of Realtime
type RealtimeStatus struct {
	// conditions represent the current state of the Realtime resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=realtimes,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Realtime is the Schema for the realtimes API
type Realtime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RealtimeSpec   `json:"spec,omitempty"`
	Status RealtimeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RealtimeList contains a list of Realtime
type RealtimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Realtime `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Realtime{}, &RealtimeList{})
}
