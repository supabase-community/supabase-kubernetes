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

// RestSpec defines the desired state of the Rest component.
type RestSpec struct {
	WorkloadConfig `json:",inline"`

	// Replicas defines the number of component instances
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// DBSchemas defines the schemas exposed by PostgREST
	// +optional
	DBSchemas *string `json:"dbSchemas,omitempty"`

	// DBMaxRows defines the maximum number of rows returned from a view, table, or stored procedure
	// +kubebuilder:validation:Minimum=1
	// +optional
	DBMaxRows *int32 `json:"dbMaxRows,omitempty"`

	// DBExtraSearchPath defines the schemas to add to the search path of every request
	// +optional
	DBExtraSearchPath *string `json:"dbExtraSearchPath,omitempty"`
}
