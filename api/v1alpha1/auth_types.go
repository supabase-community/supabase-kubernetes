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

	// Enable defines whether the Auth component is enabled
	// +optional
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`

	// Replicas defines the number of component instances
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// SiteURL is the base URL of the site used for email links and redirects
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SiteURL string `json:"siteUrl"`

	// AdditionalRedirectURLs is a list of additional URLs allowed for redirects
	// +optional
	AdditionalRedirectURLs []string `json:"additionalRedirectUrls,omitempty"`

	// DisableSignup disables new user signups
	// +optional
	// +kubebuilder:default=false
	DisableSignup *bool `json:"disableSignup,omitempty"`

	// EnableEmailSignup enables email/password signups
	// +optional
	// +kubebuilder:default=true
	EnableEmailSignup *bool `json:"enableEmailSignup,omitempty"`

	// EnableAnonymousUsers enables anonymous user signings
	// +optional
	// +kubebuilder:default=false
	EnableAnonymousUsers *bool `json:"enableAnonymousUsers,omitempty"`

	// EnableEmailAutoconfirm skips email confirmation
	// +optional
	// +kubebuilder:default=false
	EnableEmailAutoconfirm *bool `json:"enableEmailAutoconfirm,omitempty"`

	// EnablePhoneSignup enables phone signups
	// +optional
	// +kubebuilder:default=true
	EnablePhoneSignup *bool `json:"enablePhoneSignup,omitempty"`

	// EnablePhoneAutoconfirm skips phone confirmation
	// +optional
	// +kubebuilder:default=true
	EnablePhoneAutoconfirm *bool `json:"enablePhoneAutoconfirm,omitempty"`

	// SkipNonceCheck skips nonce check for external providers
	// +optional
	SkipNonceCheck *bool `json:"skipNonceCheck,omitempty"`

	// EnableMailerSecureEmailChange enables secure email change flow
	// +optional
	EnableMailerSecureEmailChange *bool `json:"enableMailerSecureEmailChange,omitempty"`

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
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// Port defines the SMTP server port
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// User defines the SMTP authentication user
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	User string `json:"user"`

	// PasswordRef references the secret containing the SMTP password
	// +kubebuilder:validation:Required
	PasswordRef SecretKeyRef `json:"passwordRef"`

	// SenderName defines the display name for outgoing emails
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SenderName string `json:"senderName"`

	// AdminEmail defines the admin email address
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AdminEmail string `json:"adminEmail"`

	// MaxFrequency defines the maximum frequency for sending emails
	// +optional
	MaxFrequency *string `json:"maxFrequency,omitempty"`
}

// OAuthProviderConfig defines a single OAuth provider configuration.
type OAuthProviderConfig struct {
	// Enable defines whether the OAuth provider is enabled
	// +optional
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`

	// ClientID defines the OAuth client ID
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
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
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// OTPExp defines the OTP expiration time in seconds
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	OTPExp int32 `json:"otpExp"`

	// OTPLength defines the length of the OTP
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=4
	// +kubebuilder:validation:Maximum=10
	OTPLength int32 `json:"otpLength"`

	// MaxFrequency defines the maximum frequency for sending SMS messages
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	MaxFrequency string `json:"maxFrequency"`

	// Template defines the SMS message template
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Template string `json:"template"`

	// Twilio defines the Twilio provider configuration
	// +optional
	Twilio *TwilioConfig `json:"twilio,omitempty"`
}

// TwilioConfig defines Twilio provider settings.
type TwilioConfig struct {
	// AccountSID defines the Twilio account SID
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AccountSID string `json:"accountSid"`

	// AuthTokenRef references the secret containing the Twilio auth token
	// +kubebuilder:validation:Required
	AuthTokenRef SecretKeyRef `json:"authTokenRef"`

	// MessageServiceSID defines the Twilio message service SID
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	MessageServiceSID string `json:"messageServiceSid"`
}

// MFAConfig defines multi-factor authentication settings.
type MFAConfig struct {
	// EnableTOTPEnroll defines whether TOTP enrollment is enabled
	// +optional
	EnableTOTPEnroll *bool `json:"enableTotpEnroll,omitempty"`

	// EnableTOTPVerify defines whether TOTP verification is enabled
	// +optional
	EnableTOTPVerify *bool `json:"enableTotpVerify,omitempty"`

	// EnablePhoneEnroll defines whether phone MFA enrollment is enabled
	// +optional
	EnablePhoneEnroll *bool `json:"enablePhoneEnroll,omitempty"`

	// EnablePhoneVerify defines whether phone MFA verification is enabled
	// +optional
	EnablePhoneVerify *bool `json:"enablePhoneVerify,omitempty"`

	// MaxEnrolledFactors defines the maximum number of enrolled MFA factors
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxEnrolledFactors *int32 `json:"maxEnrolledFactors,omitempty"`
}

// SAMLConfig defines SAML authentication settings.
type SAMLConfig struct {
	// Enable defines whether SAML authentication is enabled
	// +optional
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`

	// AllowEncryptedAssertions defines whether encrypted SAML assertions are allowed
	// +optional
	AllowEncryptedAssertions *bool `json:"allowEncryptedAssertions,omitempty"`

	// RelayStateValidityPeriod defines the validity period for the relay state
	// +optional
	RelayStateValidityPeriod *string `json:"relayStateValidityPeriod,omitempty"`

	// RateLimitAssertion defines the rate limit for SAML assertions
	// +optional
	// +kubebuilder:validation:Minimum=1
	RateLimitAssertion *int32 `json:"rateLimitAssertion,omitempty"`
}
