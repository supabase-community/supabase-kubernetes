package v1alpha1

// ExistingGatewayRef references an existing Gateway API Gateway resource.
type ExistingGatewayRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
}

// HTTPSpec defines public HTTP access settings for a Project.
type HTTPSpec struct {
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
	// +kubebuilder:validation:Required
	GatewayRef ExistingGatewayRef `json:"gatewayRef"`
}
