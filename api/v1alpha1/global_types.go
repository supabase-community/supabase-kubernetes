package v1alpha1

// GlobalSpec defines global configuration shared across all Supabase components.
type GlobalSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SiteURL string `json:"siteUrl"`
	// +optional
	AdditionalRedirectURLs []string `json:"additionalRedirectUrls,omitempty"`
	// +kubebuilder:default=3600
	// +kubebuilder:validation:Minimum=1
	// +optional
	JWTExpirySeconds *int32 `json:"jwtExpirySeconds,omitempty"`
}
