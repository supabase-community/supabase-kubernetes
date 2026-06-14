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

// FunctionSpec defines the desired state of Function.
type FunctionSpec struct {
	// ProjectRef references the Project resource that owns this function.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProjectRef string `json:"projectRef"`

	// FunctionName is the logical name of the function inside the project.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	FunctionName string `json:"functionName"`

	// Source defines the source files that make up the function.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	Source map[string]string `json:"source"`
}

// FunctionStatus defines the observed state of Function.
type FunctionStatus struct {
	// Conditions represent the latest available observations of the Function's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=functions,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Function is the Schema for the functions API.
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              FunctionSpec   `json:"spec"`
	Status            FunctionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FunctionList contains a list of Function.
type FunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Function `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Function{}, &FunctionList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (f *Function) GetConditions() *[]metav1.Condition {
	return &f.Status.Conditions
}
