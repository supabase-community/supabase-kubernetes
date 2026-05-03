package v1alpha1

// StudioSpec defines the configuration for the Supabase Studio dashboard.
type StudioSpec struct {
	ComponentSpec `json:",inline"`
	// +kubebuilder:default="Default Organization"
	// +optional
	Organization *string `json:"organization,omitempty"`
	// +kubebuilder:default="Default Project"
	// +optional
	Project *string `json:"project,omitempty"`
	// +optional
	AI *StudioAISpec `json:"ai,omitempty"`
	// +optional
	Snippets *StudioSnippetsSpec `json:"snippets,omitempty"`
}

// StudioAISpec defines AI-related configuration for Studio.
type StudioAISpec struct {
	// +optional
	APIKey *SecretKeyRef `json:"apiKey,omitempty"`
}

// StudioSnippetsSpec defines storage configuration for SQL snippets.
type StudioSnippetsSpec struct {
	// +optional
	VolumeClaimTemplate *VolumeClaimTemplateSpec `json:"volumeClaimTemplate,omitempty"`
}
