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

// StudioSpec defines the desired state of Studio.
type StudioSpec struct {
	// ProjectRef references the Project resource that owns this Studio component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProjectRef string `json:"projectRef"`

	// Replicas defines the number of Studio instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Pod defines the template for the Studio pods
	// +optional
	Pod corev1.PodTemplateSpec `json:"pod,omitempty"`

	// Service defines the template for the Studio service
	// +optional
	Service ServiceTemplate `json:"service,omitempty"`

	// Config defines Studio-specific configuration
	// +optional
	Config StudioConfig `json:"config,omitempty"`

	// Storage defines the persistent volume claims for Studio snippets
	// +optional
	Storage []corev1.PersistentVolumeClaim `json:"storage,omitempty"`
}

// StudioConfig defines Studio-specific configuration.
type StudioConfig struct {
	// OrgName defines the default organization name shown in Studio
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	OrgName string `json:"orgName"`

	// ProjName defines the default project name shown in Studio
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProjName string `json:"projName"`

	// OpenAIAPIKey references the secret containing the OpenAI API key
	// +optional
	OpenAIAPIKey *SecretKeyRef `json:"openAiApiKey,omitempty"`
}

// StudioStatus defines the observed state of Studio.
type StudioStatus struct {
	// Conditions represent the latest available observations of the Studio's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=studios,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Studio is the Schema for the studios API.
type Studio struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              StudioSpec   `json:"spec"`
	Status            StudioStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StudioList contains a list of Studio.
type StudioList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Studio `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Studio{}, &StudioList{})
}

// GetConditions returns a pointer to the status conditions slice.
func (s *Studio) GetConditions() *[]metav1.Condition {
	return &s.Status.Conditions
}
