package v1alpha1

// AuthSpec defines the configuration for the Supabase Auth (GoTrue) service.
type AuthSpec struct {
	ComponentSpec `json:",inline"`
	// +kubebuilder:default=false
	// +optional
	DisableSignup *bool `json:"disableSignup,omitempty"`
	// +kubebuilder:default=false
	// +optional
	EnableAnonymousUsers *bool `json:"enableAnonymousUsers,omitempty"`
	// +kubebuilder:default=false
	// +optional
	ExternalSkipNonceCheck *bool `json:"externalSkipNonceCheck,omitempty"`
	// +optional
	Email *AuthEmailSpec `json:"email,omitempty"`
	// +optional
	Phone *AuthPhoneSpec `json:"phone,omitempty"`
	// +optional
	OAuth *AuthOAuthSpec `json:"oauth,omitempty"`
	// +optional
	SMS *AuthSmsSpec `json:"sms,omitempty"`
	// +optional
	MFA *AuthMfaSpec `json:"mfa,omitempty"`
	// +optional
	SAML *AuthSamlSpec `json:"saml,omitempty"`
}

// AuthEmailSpec defines email authentication configuration.
type AuthEmailSpec struct {
	// +kubebuilder:default=true
	// +optional
	EnableSignup *bool `json:"enableSignup,omitempty"`
	// +kubebuilder:default=false
	// +optional
	AutoConfirm *bool `json:"autoConfirm,omitempty"`
	// +optional
	SMTP *AuthSmtpSpec `json:"smtp,omitempty"`
}

// AuthSmtpSpec defines SMTP server configuration.
type AuthSmtpSpec struct {
	// +kubebuilder:validation:Required
	AdminEmail string `json:"adminEmail"`
	// +kubebuilder:validation:Required
	Host string `json:"host"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
	// +kubebuilder:validation:Required
	UserRef SecretKeyRef `json:"userRef"`
	// +kubebuilder:validation:Required
	PassRef SecretKeyRef `json:"passRef"`
	// +optional
	SenderName *string `json:"senderName,omitempty"`
}

// AuthPhoneSpec defines phone-based authentication configuration.
type AuthPhoneSpec struct {
	// +kubebuilder:default=false
	// +optional
	EnableSignup *bool `json:"enableSignup,omitempty"`
	// +kubebuilder:default=false
	// +optional
	AutoConfirm *bool `json:"autoConfirm,omitempty"`
}

// AuthOAuthSpec defines external OAuth provider configuration.
type AuthOAuthSpec struct {
	// +optional
	Google *OAuthProviderSpec `json:"google,omitempty"`
	// +optional
	GitHub *OAuthProviderSpec `json:"github,omitempty"`
	// +optional
	Azure *OAuthProviderSpec `json:"azure,omitempty"`
}

// OAuthProviderSpec defines configuration for a single OAuth provider.
type OAuthProviderSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +optional
	ClientIDRef *SecretKeyRef `json:"clientIdRef,omitempty"`
	// +optional
	ClientSecretRef *SecretKeyRef `json:"clientSecretRef,omitempty"`
}

// AuthSmsSpec defines SMS-based authentication configuration.
type AuthSmsSpec struct {
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`
	// +optional
	OTPExpSeconds *int32 `json:"otpExpSeconds,omitempty"`
	// +optional
	OTPLength *int32 `json:"otpLength,omitempty"`
	// +optional
	MaxFrequency *string `json:"maxFrequency,omitempty"`
	// +optional
	Template *string `json:"template,omitempty"`
	// +optional
	Twilio *AuthSmsTwilioSpec `json:"twilio,omitempty"`
}

// AuthSmsTwilioSpec defines Twilio-specific SMS configuration.
type AuthSmsTwilioSpec struct {
	// +kubebuilder:validation:Required
	AccountSIDRef SecretKeyRef `json:"accountSidRef"`
	// +kubebuilder:validation:Required
	AuthTokenRef SecretKeyRef `json:"authTokenRef"`
	// +kubebuilder:validation:Required
	MessageServiceSIDRef SecretKeyRef `json:"messageServiceSidRef"`
}

// AuthMfaSpec defines multi-factor authentication configuration.
type AuthMfaSpec struct {
	// +kubebuilder:default=false
	// +optional
	TOTPEnrollEnabled *bool `json:"totpEnrollEnabled,omitempty"`
	// +kubebuilder:default=false
	// +optional
	TOTPVerifyEnabled *bool `json:"totpVerifyEnabled,omitempty"`
	// +kubebuilder:default=false
	// +optional
	PhoneEnrollEnabled *bool `json:"phoneEnrollEnabled,omitempty"`
	// +kubebuilder:default=false
	// +optional
	PhoneVerifyEnabled *bool `json:"phoneVerifyEnabled,omitempty"`
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxEnrolledFactors *int32 `json:"maxEnrolledFactors,omitempty"`
}

// AuthSamlSpec defines SAML-based authentication configuration.
type AuthSamlSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +kubebuilder:default=false
	// +optional
	AllowEncryptedAssertions *bool `json:"allowEncryptedAssertions,omitempty"`
	// +kubebuilder:default="2m0s"
	// +optional
	RelayStateValidityPeriod *string `json:"relayStateValidityPeriod,omitempty"`
	// +kubebuilder:default=15
	// +kubebuilder:validation:Minimum=0
	// +optional
	RateLimitAssertion *int32 `json:"rateLimitAssertion,omitempty"`
}
