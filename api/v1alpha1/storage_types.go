package v1alpha1

// StorageSpec defines the configuration for the Supabase Storage service.
type StorageSpec struct {
	ComponentSpec `json:",inline"`
	// +kubebuilder:default="file"
	// +kubebuilder:validation:Enum=file;s3
	// +optional
	Backend *string `json:"backend,omitempty"`
	// +kubebuilder:default="stub"
	// +optional
	Bucket *string `json:"bucket,omitempty"`
	// +kubebuilder:default="stub"
	// +optional
	TenantID *string `json:"tenantId,omitempty"`
	// +kubebuilder:default="local"
	// +optional
	Region *string `json:"region,omitempty"`
	// +kubebuilder:default=52428800
	// +kubebuilder:validation:Minimum=1
	// +optional
	FileSizeLimit *int32 `json:"fileSizeLimit,omitempty"`
	// +optional
	File *StorageFileSpec `json:"file,omitempty"`
	// +optional
	S3 *StorageS3Spec `json:"s3,omitempty"`
}

// StorageFileSpec defines configuration for file-based storage.
type StorageFileSpec struct {
	// +optional
	VolumeClaimTemplate *VolumeClaimTemplateSpec `json:"volumeClaimTemplate,omitempty"`
}

// StorageS3Spec defines configuration for S3-compatible storage.
type StorageS3Spec struct {
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`
	// +optional
	Protocol *string `json:"protocol,omitempty"`
	// +optional
	ForcePathStyle *bool `json:"forcePathStyle,omitempty"`
	// +kubebuilder:validation:Required
	AccessKeyRef SecretKeyRef `json:"accessKeyRef"`
	// +kubebuilder:validation:Required
	SecretKeyRef SecretKeyRef `json:"secretKeyRef"`
}
