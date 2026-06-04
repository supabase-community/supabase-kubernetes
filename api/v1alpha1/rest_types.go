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

// RestSpec defines the desired state of Rest
type RestSpec struct {
	WorkloadConfig `json:",inline"`

	// dbSchemas defines the schemas exposed by PostgREST
	// +kubebuilder:default="public,storage,graphql_public"
	// +optional
	DBSchemas string `json:"dbSchemas,omitempty"`

	// dbMaxRows defines the maximum number of rows returned from a view, table, or stored procedure
	// +kubebuilder:default=1000
	// +kubebuilder:validation:Minimum=1
	// +optional
	DBMaxRows *int32 `json:"dbMaxRows,omitempty"`

	// dbExtraSearchPath defines the schemas to add to the search path of every request
	// +kubebuilder:default="public"
	// +optional
	DBExtraSearchPath string `json:"dbExtraSearchPath,omitempty"`
}

// RestStatus defines the observed state of Rest
type RestStatus struct {
	// conditions represent the current state of the Rest resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rests,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Rest is the Schema for the rests API
type Rest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestSpec   `json:"spec,omitempty"`
	Status RestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestList contains a list of Rest
type RestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rest{}, &RestList{})
}
