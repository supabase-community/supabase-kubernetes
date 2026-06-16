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

// StudioSpec defines the desired state of the Studio component.
type StudioSpec struct {
	WorkloadConfig `json:",inline"`

	// Enable defines whether the Studio component is enabled
	// +optional
	// +kubebuilder:default=false
	Enable *bool `json:"enable,omitempty"`

	// Replicas defines the number of Studio instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

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

	// Storage defines the persistent volume claim for Studio snippets
	// +kubebuilder:validation:Required
	Storage VolumeClaim `json:"storage"`
}
