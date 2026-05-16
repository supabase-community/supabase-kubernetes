package v1alpha1

// HTTPConfig defines public HTTP access settings for a single endpoint.
type HTTPConfig struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=http;https
	Protocol string `json:"protocol"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// HTTPSpec defines public HTTP access settings for a Project.
type HTTPSpec struct {
	// +kubebuilder:validation:Required
	API HTTPConfig `json:"api"`
	// +kubebuilder:validation:Required
	Studio HTTPConfig `json:"studio"`
}
