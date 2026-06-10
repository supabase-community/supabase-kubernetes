/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// VolumeClaim defines the desired characteristics of a persistent volume claim.
type VolumeClaim struct {
	// +kubebuilder:validation:Required
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`

	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`

	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// DeletionPolicy defines the deletion behavior for the PVC.
// +kubebuilder:validation:Enum=Delete;Retain
type DeletionPolicy string

const (
	DeletionPolicyDelete DeletionPolicy = "Delete"
	DeletionPolicyRetain DeletionPolicy = "Retain"
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

// ResolvedDatabase exposes resolved database connection parameters.
type ResolvedDatabase struct {
	Host        string       `json:"host"`
	Port        int32        `json:"port"`
	DBName      string       `json:"dbName"`
	User        string       `json:"user,omitempty"`
	PasswordRef SecretKeyRef `json:"passwordRef"`
}

// ServiceSpec defines the configuration for a component Service.
type ServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HTTPConfig defines public HTTP access settings for a Project.
type HTTPConfig struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=http;https
	Protocol string `json:"protocol"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`
}

// WorkloadConfig defines common workload configuration shared by all Supabase components.
type WorkloadConfig struct {
	// image overrides the default component image
	// +optional
	Image string `json:"image,omitempty"`

	// imagePullPolicy defines the policy for if/when to pull the container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// replicas defines the number of component instances
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// affinity defines affinity scheduling rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// containerSecurityContext holds security configuration that will be applied to the container
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// nodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// podAnnotations defines annotations to add to the pod
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// podLabels defines labels to add to the pod
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// podSecurityContext holds pod-level security attributes
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// priorityClassName defines the priority class for the pod
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// resources defines compute resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// terminationGracePeriodSeconds defines the grace period for pod termination
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// env defines additional environment variables for the component container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// WorkloadJobConfig defines common workload configuration for Job-based resources.
type WorkloadJobConfig struct {
	// image overrides the default component image
	// +optional
	Image string `json:"image,omitempty"`

	// imagePullPolicy defines the policy for if/when to pull the container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// affinity defines affinity scheduling rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// containerSecurityContext holds security configuration that will be applied to the container
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// nodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// podAnnotations defines annotations to add to the pod
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// podLabels defines labels to add to the pod
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// podSecurityContext holds pod-level security attributes
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// priorityClassName defines the priority class for the pod
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// resources defines compute resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// terminationGracePeriodSeconds defines the grace period for pod termination
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// env defines additional environment variables for the component container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// DatabaseRef references a database resource.
type DatabaseRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=SingleDatabase
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// RestSpec defines the desired state of the Rest component.
type RestSpec struct {
	WorkloadConfig `json:",inline"`

	// dbSchemas defines the schemas exposed by PostgREST
	// +kubebuilder:default="public,storage,graphql_public"
	// +optional
	DBSchemas string `json:"dbSchemas,omitempty"`

	// dbMaxRows defines the maximum number of rows returned from a view, table, or stored procedure
	// +kubebuilder:default=1000
	// +kubebuilder:validation:Minimum=1
	// +optional
	DBMaxRows *int32 `json:"dbMaxRows,omitempty"`

	// dbExtraSearchPath defines the schemas to add to the search path of every request
	// +kubebuilder:default="public"
	// +optional
	DBExtraSearchPath string `json:"dbExtraSearchPath,omitempty"`
}

// MetaSpec defines the desired state of the Meta component.
type MetaSpec struct {
	WorkloadConfig `json:",inline"`
}

// RealtimeSpec defines the desired state of the Realtime component.
type RealtimeSpec struct {
	WorkloadConfig `json:",inline"`
}

// SMTPConfig defines SMTP settings for GoTrue.
type SMTPConfig struct {
	// +kubebuilder:validation:Required
	Host string `json:"host"`
	// +kubebuilder:validation:Required
	Port int32 `json:"port"`
	// +kubebuilder:validation:Required
	User string `json:"user"`
	// +kubebuilder:validation:Required
	PasswordRef SecretKeyRef `json:"passwordRef"`
	// +kubebuilder:validation:Required
	SenderName string `json:"senderName"`
	// +kubebuilder:validation:Required
	AdminEmail string `json:"adminEmail"`
	// +optional
	MaxFrequency string `json:"maxFrequency,omitempty"`
}

// OAuthProviderConfig defines a single OAuth provider configuration.
type OAuthProviderConfig struct {
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
	// +kubebuilder:validation:Required
	ClientID string `json:"clientId"`
	// +kubebuilder:validation:Required
	SecretRef SecretKeyRef `json:"secretRef"`
}

// OAuthConfig defines OAuth provider settings.
type OAuthConfig struct {
	// +optional
	Google *OAuthProviderConfig `json:"google,omitempty"`
	// +optional
	GitHub *OAuthProviderConfig `json:"github,omitempty"`
	// +optional
	Azure *OAuthProviderConfig `json:"azure,omitempty"`
}

// SMSConfig defines SMS provider settings.
type SMSConfig struct {
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`
	// +kubebuilder:validation:Required
	OTPExp int32 `json:"otpExp"`
	// +kubebuilder:validation:Required
	OTPLength int32 `json:"otpLength"`
	// +kubebuilder:validation:Required
	MaxFrequency string `json:"maxFrequency"`
	// +kubebuilder:validation:Required
	Template string `json:"template"`
	// +kubebuilder:validation:Required
	TwilioAccountSID string `json:"twilioAccountSid"`
	// +kubebuilder:validation:Required
	TwilioAuthTokenRef SecretKeyRef `json:"twilioAuthTokenRef"`
	// +kubebuilder:validation:Required
	TwilioMessageServiceSID string `json:"twilioMessageServiceSid"`
}

// MFAConfig defines multi-factor authentication settings.
type MFAConfig struct {
	// +optional
	TOTPEnrollEnabled bool `json:"totpEnrollEnabled,omitempty"`
	// +optional
	TOTPVerifyEnabled bool `json:"totpVerifyEnabled,omitempty"`
	// +optional
	PhoneEnrollEnabled bool `json:"phoneEnrollEnabled,omitempty"`
	// +optional
	PhoneVerifyEnabled bool `json:"phoneVerifyEnabled,omitempty"`
	// +optional
	MaxEnrolledFactors int32 `json:"maxEnrolledFactors,omitempty"`
}

// SAMLConfig defines SAML authentication settings.
type SAMLConfig struct {
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	AllowEncryptedAssertions bool `json:"allowEncryptedAssertions,omitempty"`
	// +optional
	RelayStateValidityPeriod string `json:"relayStateValidityPeriod,omitempty"`
	// +optional
	RateLimitAssertion int32 `json:"rateLimitAssertion,omitempty"`
}

// AuthSpec defines the desired state of the Auth component.
type AuthSpec struct {
	WorkloadConfig `json:",inline"`

	// siteUrl is the base URL of your site used for email links and redirects
	// +kubebuilder:validation:Required
	SiteURL string `json:"siteUrl"`

	// additionalRedirectUrls is a list of additional URLs allowed for redirects
	// +optional
	AdditionalRedirectURLs []string `json:"additionalRedirectUrls,omitempty"`

	// disableSignup disables new user signups
	// +kubebuilder:validation:Required
	DisableSignup bool `json:"disableSignup"`

	// enableEmailSignup enables email/password signups
	// +kubebuilder:validation:Required
	EnableEmailSignup bool `json:"enableEmailSignup"`

	// enableAnonymousUsers enables anonymous user signings
	// +kubebuilder:validation:Required
	EnableAnonymousUsers bool `json:"enableAnonymousUsers"`

	// enableEmailAutoconfirm skips email confirmation
	// +kubebuilder:validation:Required
	EnableEmailAutoconfirm bool `json:"enableEmailAutoconfirm"`

	// enablePhoneSignup enables phone signups
	// +kubebuilder:validation:Required
	EnablePhoneSignup bool `json:"enablePhoneSignup"`

	// enablePhoneAutoconfirm skips phone confirmation
	// +kubebuilder:validation:Required
	EnablePhoneAutoconfirm bool `json:"enablePhoneAutoconfirm"`
	// skipNonceCheck skips nonce check for external providers
	// +optional
	SkipNonceCheck *bool `json:"skipNonceCheck,omitempty"`
	// mailerSecureEmailChangeEnabled enables secure email change flow
	// +optional
	MailerSecureEmailChangeEnabled *bool `json:"mailerSecureEmailChangeEnabled,omitempty"`

	// smtp defines SMTP configuration for sending emails
	// +optional
	SMTP *SMTPConfig `json:"smtp,omitempty"`

	// oauth defines OAuth provider configuration
	// +optional
	OAuth *OAuthConfig `json:"oauth,omitempty"`

	// sms defines SMS provider configuration
	// +optional
	SMS *SMSConfig `json:"sms,omitempty"`

	// mfa defines multi-factor authentication configuration
	// +optional
	MFA *MFAConfig `json:"mfa,omitempty"`

	// saml defines SAML authentication configuration
	// +optional
	SAML *SAMLConfig `json:"saml,omitempty"`
}
