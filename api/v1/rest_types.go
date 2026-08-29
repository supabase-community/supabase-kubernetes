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

// RestSpec defines the desired state of Rest.
type RestSpec struct {
	// ProjectRef references the Project resource that owns this Rest component.
	// +kubebuilder:validation:Required
	ProjectRef corev1.LocalObjectReference `json:"projectRef"`

	// Replicas defines the number of Rest instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the Rest pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the Rest service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines Rest-specific configuration
	// +optional
	Config RestConfig `json:"config,omitempty"`
}

// RestConfig defines Rest-specific configuration.
type RestConfig struct {
	// DBSchemas defines the schemas exposed by PostgREST
	// +optional
	// +kubebuilder:default="public,storage,graphql_public"
	DBSchemas *string `json:"dbSchemas,omitempty"`

	// DBMaxRows defines the maximum number of rows returned from a view, table, or stored procedure
	// +optional
	// +kubebuilder:default=1000
	// +kubebuilder:validation:Minimum=1
	DBMaxRows *int32 `json:"dbMaxRows,omitempty"`

	// DBExtraSearchPath defines the schemas to add to the search path of every request
	// +optional
	// +kubebuilder:default="public"
	DBExtraSearchPath *string `json:"dbExtraSearchPath,omitempty"`
}

// RestStatus defines the observed state of Rest.
type RestStatus struct {
	// Conditions represent the latest available observations of the Rest's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rests,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Rest is the Schema for the rests API.
type Rest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RestSpec   `json:"spec"`
	Status            RestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestList contains a list of Rest.
type RestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rest{}, &RestList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (r *Rest) GetConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
