package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// SecretKeyRef is a reference to a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ComponentProbes defines custom probe overrides for a component.
type ComponentProbes struct {
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
}

// ComponentSpec defines the common fields shared by all Supabase service components.
type ComponentSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Probes *ComponentProbes `json:"probes,omitempty"`
}

// VolumeClaimTemplateSpec defines the desired characteristics of a persistent volume claim.
type VolumeClaimTemplateSpec struct {
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +kubebuilder:validation:Required
	Resources corev1.VolumeResourceRequirements `json:"resources"`
}
