package v1alpha1

// RestSpec defines the configuration for the Supabase PostgREST API service.
type RestSpec struct {
	ComponentSpec `json:",inline"`
	// +kubebuilder:default={"public","storage","graphql_public"}
	// +optional
	DBSchemas []string `json:"dbSchemas,omitempty"`
	// +kubebuilder:default=1000
	// +kubebuilder:validation:Minimum=1
	// +optional
	DBMaxRows *int32 `json:"dbMaxRows,omitempty"`
	// +kubebuilder:default="public"
	// +optional
	DBExtraSearchPath *string `json:"dbExtraSearchPath,omitempty"`
}
