package v1alpha1

// GatewaySpec defines the Gateway API configuration for routing traffic.
type GatewaySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	GatewayClassName string `json:"gatewayClassName"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Listeners []GatewayListenerSpec `json:"listeners"`
}

// GatewayListenerSpec defines a single Gateway listener.
type GatewayListenerSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Protocol string `json:"protocol"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}
