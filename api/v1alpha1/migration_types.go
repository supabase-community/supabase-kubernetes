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

// MigrationSpec defines the desired state of Migration.
// +kubebuilder:validation:XValidation:rule="self.migrations == oldSelf.migrations",message="migrations are immutable after creation"
type MigrationSpec struct {
	WorkloadConfig `json:",inline"`

	// DatabaseRef references the database resource
	// +kubebuilder:validation:Required
	DatabaseRef DatabaseRef `json:"databaseRef"`

	// Migrations is the ordered list of migration steps to apply
	// The entire array is immutable after creation
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Migrations []MigrationEntry `json:"migrations"`
}

// MigrationStatus defines the observed state of Migration.
type MigrationStatus struct {
	// Conditions represent the latest available observations of the Migration's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AppliedHash is the SHA-256 hash of the batch that was successfully applied
	// +optional
	AppliedHash string `json:"appliedHash,omitempty"`

	// AppliedAt is when the batch was successfully applied
	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=migrations,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Applied Hash",type=string,JSONPath=`.status.appliedHash`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Migration is the Schema for the migrations API.
type Migration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MigrationSpec   `json:"spec"`
	Status            MigrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationList contains a list of Migration.
type MigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Migration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Migration{}, &MigrationList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (m *Migration) GetConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

// MigrationEntry defines a single ordered migration step.
type MigrationEntry struct {
	// Name is a human-readable identifier for this migration step
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// SQL is the migration script to execute
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=65536
	SQL string `json:"sql"`
}
