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

// ProjectSpec defines the desired state of Project.
type ProjectSpec struct {
	// JWTExpSec defines the JWT expiration time in seconds.
	// +optional
	// +kubebuilder:default=3600
	// +kubebuilder:validation:Minimum=1
	JWTExpSec *int32 `json:"jwtExpSec,omitempty"`

	// PublicURL defines the public URL for the Project.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PublicURL string `json:"publicUrl"`

	// DatabaseRef references the database resource.
	// +kubebuilder:validation:Required
	DatabaseRef DatabaseRef `json:"databaseRef"`
}

// ProjectStatus defines the observed state of Project.
type ProjectStatus struct {
	// Conditions include Ready and its current reconciliation reason.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// JWTSyncHash is the hash of the JWT configuration last successfully applied by the sync job.
	// +optional
	JWTSyncHash string `json:"jwtSyncHash,omitempty"`

	// PasswordSyncHash is the hash of the password configuration last successfully applied by the sync job.
	// +optional
	PasswordSyncHash string `json:"passwordSyncHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=projects,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Project is the Schema for the projects API.
type Project struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Project.
	// +required
	Spec ProjectSpec `json:"spec"`

	// Status defines the observed state of Project.
	// +optional
	Status ProjectStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}

func (c *Project) GetConditions() *[]metav1.Condition { return &c.Status.Conditions }
