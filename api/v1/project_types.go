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

// ProjectSpec defines the desired state of a Supabase deployment.
type ProjectSpec struct {
	// JWTExpSec defines the JWT expiration time in seconds
	// +optional
	// +kubebuilder:default=3600
	// +kubebuilder:validation:Minimum=1
	JWTExpSec *int32 `json:"jwtExpSec,omitempty"`

	// APIURL defines the public API URL for the Project
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	APIURL string `json:"apiUrl"`

	// DatabaseRef references the database resource
	// +kubebuilder:validation:Required
	DatabaseRef DatabaseReference `json:"databaseRef"`

	// Migration defines the template used for the initial Migration created by the Project.
	// +optional
	Migration *MigrationTemplateSpec `json:"migration,omitempty"`

	// SyncJWTJob defines the template used for the JWT sync Job created by the Project.
	// +optional
	SyncJWTJob *JobTemplateSpec `json:"syncJwtJob,omitempty"`

	// SyncPasswordJob defines the template used for the password sync Job created by the Project.
	// +optional
	SyncPasswordJob *JobTemplateSpec `json:"syncPasswordJob,omitempty"`
}

// MigrationTemplateSpec exposes the user-configurable parts of a MigrationSpec
// when the Migration is owned by a Project.
type MigrationTemplateSpec struct {
	// Pod is the template for the migration Job pods.
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`
}

// JobTemplateSpec defines the template for a Job created by the Project.
type JobTemplateSpec struct {
	// Pod is the template for the Job pods.
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`
}

// ProjectStatus defines the observed state of a Supabase deployment.
type ProjectStatus struct {
	// Conditions represent the latest available observations of the Project's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// JwtSyncHash is the hash of the JWT configuration last successfully applied by the sync job.
	// +optional
	JwtSyncHash string `json:"jwtSyncHash,omitempty"`

	// PasswordSyncHash is the hash of the password configuration last successfully applied by the sync job.
	// +optional
	PasswordSyncHash string `json:"passwordSyncHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=projects,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="API URL",type=string,JSONPath=`.spec.apiUrl`
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
