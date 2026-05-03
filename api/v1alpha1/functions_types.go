package v1alpha1

// FunctionsSpec defines the configuration for the Supabase Edge Functions service.
type FunctionsSpec struct {
	ComponentSpec `json:",inline"`
	// +kubebuilder:default=false
	// +optional
	VerifyJWT *bool `json:"verifyJwt,omitempty"`
}
