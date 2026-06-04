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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// AuthSpec defines the desired state of Auth
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

// AuthStatus defines the observed state of Auth.
type AuthStatus struct {
	// conditions represent the current state of the Auth resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=auths,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Auth is the Schema for the auths API
type Auth struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthSpec   `json:"spec,omitempty"`
	Status AuthStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthList contains a list of Auth
type AuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Auth `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Auth{}, &AuthList{})
}
