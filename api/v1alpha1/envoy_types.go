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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvoySpec defines the desired state of the Envoy component.
type EnvoySpec struct {
	// ProjectRef references a Project in the same namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0",message="projectRef.name is required"
	ProjectRef corev1.LocalObjectReference `json:"projectRef"`

	// Pod overlays the operator-generated Pod template.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Pod *corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Replicas defines the number of component instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Service defines the configuration for the component Service
	// +optional
	Service *ServiceTemplate `json:"service,omitempty"`

	// Config defines extra environment variables merged into the Envoy containers.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Config []corev1.EnvVar `json:"config,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// EnvoyStatus defines the observed state of Envoy.
type EnvoyStatus struct {
	// Conditions include Ready and its current reconciliation reason.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Envoy is the Schema for the envoys API
type Envoy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Envoy
	// +required
	Spec EnvoySpec `json:"spec"`

	// status defines the observed state of Envoy
	// +optional
	Status EnvoyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// EnvoyList contains a list of Envoy
type EnvoyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Envoy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Envoy{}, &EnvoyList{})
}

func (c *Envoy) GetConditions() *[]metav1.Condition         { return &c.Status.Conditions }
func (c *Envoy) GetProjectRef() corev1.LocalObjectReference { return c.Spec.ProjectRef }
