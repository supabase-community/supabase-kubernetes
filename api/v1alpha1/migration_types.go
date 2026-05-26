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

// MigrationEntry defines a single ordered migration step.
type MigrationEntry struct {
	// Name is a unique identifier for this migration step.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// SQL is the migration script to execute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=65536
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="sql is immutable once created"
	SQL string `json:"sql"`
}

// MigrationSpec defines the desired state of Migration.
// +kubebuilder:validation:XValidation:rule="size(self.migrations) >= size(oldSelf.migrations)",message="migrations cannot be removed"
type MigrationSpec struct {
	// +kubebuilder:validation:Required
	DatabaseRef DatabaseRef `json:"databaseRef"`
	// Image to use for migration jobs (must contain psql).
	// +kubebuilder:validation:Required
	Image string `json:"image"`
	// Migrations is the ordered list of migration steps to apply sequentially.
	// Existing entries are immutable; new entries may only be appended.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +listType=map
	// +listMapKey=name
	Migrations []MigrationEntry `json:"migrations"`
}

// MigrationStepStatus tracks the status of a single migration step.
type MigrationStepStatus struct {
	// Name of the migration entry.
	Name string `json:"name"`
	// Applied indicates if this migration was successfully applied.
	Applied bool `json:"applied"`
	// AppliedAt is when the migration was applied.
	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`
	// JobName is the Job that executed this migration.
	// +optional
	JobName string `json:"jobName,omitempty"`
}

// MigrationStatus defines the observed state of Migration.
type MigrationStatus struct {
	// MigrationStatuses tracks the status of each individual migration step.
	// +optional
	MigrationStatuses []MigrationStepStatus `json:"migrationStatuses,omitempty"`
	// conditions represent the current state of the Migration resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=migrations,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Migration is the Schema for the migrations API.
type Migration struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Migration
	// +required
	Spec MigrationSpec `json:"spec"`

	// status defines the observed state of Migration
	// +optional
	Status MigrationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MigrationList contains a list of Migration
type MigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Migration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Migration{}, &MigrationList{})
}
