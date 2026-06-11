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

// ProjectSpec defines the desired state of a Supabase deployment.
type ProjectSpec struct {
	// JWTExpSec defines the JWT expiration time in seconds
	// +kubebuilder:validation:Minimum=1
	// +optional
	JWTExpSec *int32 `json:"jwtExpSec,omitempty"`

	// HTTP defines the public HTTP access settings
	// +kubebuilder:validation:Required
	HTTP HTTPConfig `json:"http"`

	// DatabaseRef references the database resource
	// +kubebuilder:validation:Required
	DatabaseRef DatabaseRef `json:"databaseRef"`

	// Rest defines the Rest component configuration
	// +optional
	Rest *RestSpec `json:"rest,omitempty"`

	// Meta defines the Meta component configuration
	// +optional
	Meta *MetaSpec `json:"meta,omitempty"`

	// Realtime defines the Realtime component configuration
	// +optional
	Realtime *RealtimeSpec `json:"realtime,omitempty"`

	// Auth defines the Auth component configuration
	// +optional
	Auth *AuthSpec `json:"auth,omitempty"`
}

// ProjectStatus defines the observed state of a Supabase deployment.
type ProjectStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=projects,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Project is the Schema for the projects API.
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectSpec   `json:"spec"`
	Status            ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (p *Project) GetConditions() *[]metav1.Condition {
	return &p.Status.Conditions
}
