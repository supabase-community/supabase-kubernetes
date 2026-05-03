package v1alpha1

// DatabaseSpec defines connection details for the external PostgreSQL database.
type DatabaseSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// +kubebuilder:default=5432
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
	// +kubebuilder:default="postgres"
	// +optional
	DBName *string `json:"dbName,omitempty"`
	// +kubebuilder:validation:Required
	PasswordRef SecretKeyRef `json:"passwordRef"`
}
