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

// AuthSpec defines the desired state of the Auth component.
type AuthSpec struct {
	WorkloadConfig `json:",inline"`

	// Replicas defines the number of component instances
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// SiteURL is the base URL of your site used for email links and redirects
	// +kubebuilder:validation:Required
	SiteURL string `json:"siteUrl"`

	// AdditionalRedirectURLs is a list of additional URLs allowed for redirects
	// +optional
	AdditionalRedirectURLs []string `json:"additionalRedirectUrls,omitempty"`

	// DisableSignup disables new user signups
	// +kubebuilder:validation:Required
	DisableSignup bool `json:"disableSignup"`

	// EnableEmailSignup enables email/password signups
	// +kubebuilder:validation:Required
	EnableEmailSignup bool `json:"enableEmailSignup"`

	// EnableAnonymousUsers enables anonymous user signings
	// +kubebuilder:validation:Required
	EnableAnonymousUsers bool `json:"enableAnonymousUsers"`

	// EnableEmailAutoconfirm skips email confirmation
	// +kubebuilder:validation:Required
	EnableEmailAutoconfirm bool `json:"enableEmailAutoconfirm"`

	// EnablePhoneSignup enables phone signups
	// +kubebuilder:validation:Required
	EnablePhoneSignup bool `json:"enablePhoneSignup"`

	// EnablePhoneAutoconfirm skips phone confirmation
	// +kubebuilder:validation:Required
	EnablePhoneAutoconfirm bool `json:"enablePhoneAutoconfirm"`

	// SkipNonceCheck skips nonce check for external providers
	// +optional
	SkipNonceCheck *bool `json:"skipNonceCheck,omitempty"`

	// MailerSecureEmailChangeEnabled enables secure email change flow
	// +optional
	MailerSecureEmailChangeEnabled *bool `json:"mailerSecureEmailChangeEnabled,omitempty"`

	// SMTP defines SMTP configuration for sending emails
	// +optional
	SMTP *SMTPConfig `json:"smtp,omitempty"`

	// OAuth defines OAuth provider configuration
	// +optional
	OAuth *OAuthConfig `json:"oauth,omitempty"`

	// SMS defines SMS provider configuration
	// +optional
	SMS *SMSConfig `json:"sms,omitempty"`

	// MFA defines multi-factor authentication configuration
	// +optional
	MFA *MFAConfig `json:"mfa,omitempty"`

	// SAML defines SAML authentication configuration
	// +optional
	SAML *SAMLConfig `json:"saml,omitempty"`
}

// SMTPConfig defines SMTP settings for GoTrue.
type SMTPConfig struct {
	// Host defines the SMTP server host
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Port defines the SMTP server port
	// +kubebuilder:validation:Required
	Port int32 `json:"port"`

	// User defines the SMTP authentication user
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// PasswordRef references the secret containing the SMTP password
	// +kubebuilder:validation:Required
	PasswordRef SecretKeyRef `json:"passwordRef"`

	// SenderName defines the display name for outgoing emails
	// +kubebuilder:validation:Required
	SenderName string `json:"senderName"`

	// AdminEmail defines the admin email address
	// +kubebuilder:validation:Required
	AdminEmail string `json:"adminEmail"`

	// MaxFrequency defines the maximum frequency for sending emails
	// +optional
	MaxFrequency *string `json:"maxFrequency,omitempty"`
}

// OAuthProviderConfig defines a single OAuth provider configuration.
type OAuthProviderConfig struct {
	// Enabled defines whether the OAuth provider is enabled
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// ClientID defines the OAuth client ID
	// +kubebuilder:validation:Required
	ClientID string `json:"clientId"`

	// SecretRef references the secret containing the OAuth client secret
	// +kubebuilder:validation:Required
	SecretRef SecretKeyRef `json:"secretRef"`
}

// OAuthConfig defines OAuth provider settings.
type OAuthConfig struct {
	// Google defines the Google OAuth provider configuration
	// +optional
	Google *OAuthProviderConfig `json:"google,omitempty"`

	// GitHub defines the GitHub OAuth provider configuration
	// +optional
	GitHub *OAuthProviderConfig `json:"github,omitempty"`

	// Azure defines the Azure OAuth provider configuration
	// +optional
	Azure *OAuthProviderConfig `json:"azure,omitempty"`
}

// SMSConfig defines SMS provider settings.
type SMSConfig struct {
	// Provider defines the SMS provider name
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`

	// OTPExp defines the OTP expiration time in seconds
	// +kubebuilder:validation:Required
	OTPExp int32 `json:"otpExp"`

	// OTPLength defines the length of the OTP
	// +kubebuilder:validation:Required
	OTPLength int32 `json:"otpLength"`

	// MaxFrequency defines the maximum frequency for sending SMS messages
	// +kubebuilder:validation:Required
	MaxFrequency string `json:"maxFrequency"`

	// Template defines the SMS message template
	// +kubebuilder:validation:Required
	Template string `json:"template"`

	// TwilioAccountSID defines the Twilio account SID
	// +kubebuilder:validation:Required
	TwilioAccountSID string `json:"twilioAccountSid"`

	// TwilioAuthTokenRef references the secret containing the Twilio auth token
	// +kubebuilder:validation:Required
	TwilioAuthTokenRef SecretKeyRef `json:"twilioAuthTokenRef"`

	// TwilioMessageServiceSID defines the Twilio message service SID
	// +kubebuilder:validation:Required
	TwilioMessageServiceSID string `json:"twilioMessageServiceSid"`
}

// MFAConfig defines multi-factor authentication settings.
type MFAConfig struct {
	// TOTPEnrollEnabled defines whether TOTP enrollment is enabled
	// +optional
	TOTPEnrollEnabled *bool `json:"totpEnrollEnabled,omitempty"`

	// TOTPVerifyEnabled defines whether TOTP verification is enabled
	// +optional
	TOTPVerifyEnabled *bool `json:"totpVerifyEnabled,omitempty"`

	// PhoneEnrollEnabled defines whether phone MFA enrollment is enabled
	// +optional
	PhoneEnrollEnabled *bool `json:"phoneEnrollEnabled,omitempty"`

	// PhoneVerifyEnabled defines whether phone MFA verification is enabled
	// +optional
	PhoneVerifyEnabled *bool `json:"phoneVerifyEnabled,omitempty"`

	// MaxEnrolledFactors defines the maximum number of enrolled MFA factors
	// +optional
	MaxEnrolledFactors *int32 `json:"maxEnrolledFactors,omitempty"`
}

// SAMLConfig defines SAML authentication settings.
type SAMLConfig struct {
	// Enabled defines whether SAML authentication is enabled
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// AllowEncryptedAssertions defines whether encrypted SAML assertions are allowed
	// +optional
	AllowEncryptedAssertions *bool `json:"allowEncryptedAssertions,omitempty"`

	// RelayStateValidityPeriod defines the validity period for the relay state
	// +optional
	RelayStateValidityPeriod *string `json:"relayStateValidityPeriod,omitempty"`

	// RateLimitAssertion defines the rate limit for SAML assertions
	// +optional
	RateLimitAssertion *int32 `json:"rateLimitAssertion,omitempty"`
}
