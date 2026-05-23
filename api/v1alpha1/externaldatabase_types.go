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

// ExternalDatabaseSpec defines the desired state of ExternalDatabase.
type ExternalDatabaseSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// +kubebuilder:default=5432
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
	// +kubebuilder:default="postgres"
	// +optional
	DBName *string `json:"dbName,omitempty"`
	// +kubebuilder:validation:Required
	PasswordRef SecretKeyRef `json:"passwordRef"`
}

// ExternalDatabaseStatus defines the observed state of ExternalDatabase.
type ExternalDatabaseStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=externaldatabases,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ExternalDatabase is the Schema for the externaldatabases API.
type ExternalDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ExternalDatabaseSpec   `json:"spec,omitempty"`
	Status            ExternalDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalDatabaseList contains a list of ExternalDatabase.
type ExternalDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalDatabase{}, &ExternalDatabaseList{})
}
