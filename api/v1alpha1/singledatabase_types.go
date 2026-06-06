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

// SingleDatabaseSpec defines the desired state of SingleDatabase.
type SingleDatabaseSpec struct {
	WorkloadConfig `json:",inline"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`

	// +optional
	Storage VolumeClaimTemplateSpec `json:"storage,omitempty"`
}

// SingleDatabaseStatus defines the observed state of SingleDatabase.
type SingleDatabaseStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	ResolvedDatabase *ResolvedDatabase `json:"resolvedDatabase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=singledatabases,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SingleDatabase is the Schema for the singledatabases API.
type SingleDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SingleDatabaseSpec   `json:"spec,omitempty"`
	Status            SingleDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SingleDatabaseList contains a list of SingleDatabase.
type SingleDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SingleDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SingleDatabase{}, &SingleDatabaseList{})
}
