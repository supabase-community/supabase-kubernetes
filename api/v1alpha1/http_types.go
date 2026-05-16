package v1alpha1

import corev1 "k8s.io/api/core/v1"

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
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// HTTPSpec defines public HTTP access settings for a Project.
type HTTPSpec struct {
	// +kubebuilder:validation:Required
	API HTTPConfig `json:"api"`
	// +kubebuilder:validation:Required
	Studio HTTPConfig `json:"studio"`
}
