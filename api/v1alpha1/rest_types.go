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

	// replicas defines the number of component instances
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// service defines the configuration for the component Service
	// +kubebuilder:default={}
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

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
