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

// StorageSpec defines the desired state of the Storage component.
type StorageSpec struct {
	WorkloadConfig `json:",inline"`

	// Enable defines whether the Storage component is enabled
	// +optional
	// +kubebuilder:default=false
	Enable *bool `json:"enable,omitempty"`

	// Replicas defines the number of Storage instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// FileSizeLimit defines the maximum file size in bytes
	// +optional
	// +kubebuilder:default=52428800
	// +kubebuilder:validation:Minimum=1
	FileSizeLimit *int64 `json:"fileSizeLimit,omitempty"`

	// Storage defines the persistent volume claim for Storage data
	// +kubebuilder:validation:Required
	Storage VolumeClaim `json:"storage"`
}
