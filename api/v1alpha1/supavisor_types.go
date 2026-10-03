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

// SupavisorSpec defines the desired state of the Supavisor component.
type SupavisorSpec struct {
	// ProjectRef references a Project in the same namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0",message="projectRef.name is required"
	// +kubebuilder:validation:XValidation:rule="size(self.name) <= 253 && self.name.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$')",message="projectRef.name must be a valid DNS subdomain"
	ProjectRef corev1.LocalObjectReference `json:"projectRef"`

	// Pod overlays the operator-generated Pod template.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Pod *corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the configuration for the component Service.
	// +optional
	Service *ServiceTemplate `json:"service,omitempty"`

	// Config defines extra environment variables merged into the Supavisor container.
	// +listType=map
	// +listMapKey=name
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Config []corev1.EnvVar `json:"config,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// SupavisorStatus defines the observed state of Supavisor.
type SupavisorStatus struct {
	// Conditions include Ready and its current reconciliation reason.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=supavisors,scope=Namespaced
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Supavisor is the Schema for the supavisors API.
type Supavisor struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Supavisor.
	// +required
	Spec SupavisorSpec `json:"spec"`

	// Status defines the observed state of Supavisor.
	// +optional
	Status SupavisorStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SupavisorList contains a list of Supavisor.
type SupavisorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Supavisor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Supavisor{}, &SupavisorList{})
}

func (c *Supavisor) GetConditions() *[]metav1.Condition         { return &c.Status.Conditions }
func (c *Supavisor) GetProjectRef() corev1.LocalObjectReference { return c.Spec.ProjectRef }
