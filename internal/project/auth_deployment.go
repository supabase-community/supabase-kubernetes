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

package project

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// AuthDeploymentName returns the name of the Auth Deployment for a Project.
func AuthDeploymentName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-auth", project.Name)
}

// AuthDeployment constructs the Auth Deployment for a Project.
func AuthDeployment(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.Deployment, error) {
	if project.Spec.Auth == nil || !*project.Spec.Auth.Enable {
		return nil, nil
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    AuthLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: authReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: AuthSelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      authPodLabels(project),
					Annotations: authPodAnnotations(project),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Auth.Affinity,
					NodeSelector:                  project.Spec.Auth.NodeSelector,
					Tolerations:                   project.Spec.Auth.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Auth.PriorityClassName, ""),
					SecurityContext:               project.Spec.Auth.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Auth.TerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						buildAuthContainer(project, db),
					},
				},
			},
		},
	}

	return deploy, nil
}

// authReplicas returns the number of Auth replicas from the spec or the default.
func authReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Auth != nil && project.Spec.Auth.Replicas != nil {
		return project.Spec.Auth.Replicas
	}
	return ptr.To(int32(1))
}

// authPodLabels returns the merged pod labels for the Auth component.
func authPodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(AuthLabels(project))
	maps.Copy(labels, project.Spec.Auth.PodLabels)
	return labels
}

// authPodAnnotations returns the merged pod annotations for the Auth component.
func authPodAnnotations(project *supabasev1alpha1.Project) map[string]string {
	return project.Spec.Auth.PodAnnotations
}

// buildAuthContainer returns the Auth container specification.
func buildAuthContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "auth",
		Image:           authImage(project),
		ImagePullPolicy: authImagePullPolicy(project),
		Env:             buildAuthEnvVars(project, db),
		Ports:           authPorts(),
		Resources:       ptr.Deref(project.Spec.Auth.Resources, corev1.ResourceRequirements{}),
		LivenessProbe:   authLivenessProbe(),
		ReadinessProbe:  authReadinessProbe(),
		StartupProbe:    authStartupProbe(),
	}
}

// authImage returns the Auth image from the spec or the default image.
func authImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Auth.Image != nil && *project.Spec.Auth.Image != "" {
		return *project.Spec.Auth.Image
	}
	return DefaultAuthImage
}

// authImagePullPolicy returns the Auth image pull policy from the spec or the default.
func authImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Auth.ImagePullPolicy != nil {
		return *project.Spec.Auth.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// authPorts returns the container ports for the Auth container.
func authPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "auth",
			ContainerPort: DefaultAuthPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// authLivenessProbe returns the liveness probe for the Auth container.
func authLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        authProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// authReadinessProbe returns the readiness probe for the Auth container.
func authReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        authProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// authStartupProbe returns the startup probe for the Auth container.
func authStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        authProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// authProbeHandler returns the shared probe handler for Auth health checks.
func authProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"wget",
				"--no-verbose",
				"--tries=1",
				"--spider",
				fmt.Sprintf("http://localhost:%s/health", strconv.Itoa(int(DefaultAuthPort))),
			},
		},
	}
}

// buildAuthEnvVars returns the environment variables for the Auth container.
func buildAuthEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	apiURL := APIExternalURL(project)
	auth := project.Spec.Auth

	env := []corev1.EnvVar{
		helper.EnvVar("GOTRUE_API_HOST", "0.0.0.0"),
		helper.EnvVar("GOTRUE_API_PORT", strconv.Itoa(int(DefaultAuthPort))),
		helper.EnvVar("API_EXTERNAL_URL", apiURL),
		helper.EnvVar("GOTRUE_DB_DRIVER", "postgres"),
		helper.EnvVarFromSecret("DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("GOTRUE_DB_DATABASE_URL", fmt.Sprintf(
			"postgres://supabase_auth_admin:$(DB_PASSWORD)@%s:%s/%s",
			db.Host,
			strconv.Itoa(int(db.Port)),
			db.DBName,
		)),
		helper.EnvVar("GOTRUE_SITE_URL", auth.SiteURL),
		helper.EnvVar("GOTRUE_DISABLE_SIGNUP", strconv.FormatBool(*auth.DisableSignup)),
		helper.EnvVar("GOTRUE_JWT_ADMIN_ROLES", "service_role"),
		helper.EnvVar("GOTRUE_JWT_AUD", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_DEFAULT_GROUP_NAME", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_EXP", strconv.Itoa(int(*project.Spec.JWTExpSec))),
		helper.EnvVarFromSecret("GOTRUE_JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVarFromSecret("GOTRUE_JWT_KEYS", JWTSecretName(project), JWTSecretKeys),
		helper.EnvVar("GOTRUE_JWT_ISSUER", fmt.Sprintf("%s/auth/v1", apiURL)),
		helper.EnvVar("GOTRUE_EXTERNAL_EMAIL_ENABLED", strconv.FormatBool(*auth.EnableEmailSignup)),
		helper.EnvVar("GOTRUE_EXTERNAL_ANONYMOUS_USERS_ENABLED", strconv.FormatBool(*auth.EnableAnonymousUsers)),
		helper.EnvVar("GOTRUE_MAILER_AUTOCONFIRM", strconv.FormatBool(*auth.EnableEmailAutoconfirm)),
		helper.EnvVar("GOTRUE_EXTERNAL_PHONE_ENABLED", strconv.FormatBool(*auth.EnablePhoneSignup)),
		helper.EnvVar("GOTRUE_SMS_AUTOCONFIRM", strconv.FormatBool(*auth.EnablePhoneAutoconfirm)),
	}

	if len(auth.AdditionalRedirectURLs) > 0 {
		env = append(env, helper.EnvVar("GOTRUE_URI_ALLOW_LIST", strings.Join(auth.AdditionalRedirectURLs, ",")))
	}

	if auth.SkipNonceCheck != nil {
		env = append(env, helper.EnvVar("GOTRUE_EXTERNAL_SKIP_NONCE_CHECK", strconv.FormatBool(*auth.SkipNonceCheck)))
	}

	if auth.EnableMailerSecureEmailChange != nil {
		env = append(env, helper.EnvVar("GOTRUE_MAILER_SECURE_EMAIL_CHANGE_ENABLED", strconv.FormatBool(*auth.EnableMailerSecureEmailChange)))
	}

	env = append(env, buildAuthSMTPEnvVars(project)...)
	env = append(env, buildAuthOAuthEnvVars(project, apiURL)...)
	env = append(env, buildAuthSMSEnvVars(project)...)
	env = append(env, buildAuthMFAEnvVars(project)...)
	env = append(env, buildAuthSAMLEnvVars(project)...)

	return env
}

// buildAuthSMTPEnvVars returns the SMTP environment variables for the Auth container.
func buildAuthSMTPEnvVars(project *supabasev1alpha1.Project) []corev1.EnvVar {
	if project.Spec.Auth.SMTP == nil {
		return nil
	}

	smtp := project.Spec.Auth.SMTP
	env := []corev1.EnvVar{
		helper.EnvVar("GOTRUE_SMTP_ADMIN_EMAIL", smtp.AdminEmail),
		helper.EnvVar("GOTRUE_SMTP_HOST", smtp.Host),
		helper.EnvVar("GOTRUE_SMTP_PORT", strconv.Itoa(int(smtp.Port))),
		helper.EnvVar("GOTRUE_SMTP_USER", smtp.User),
		helper.EnvVarFromSecret("GOTRUE_SMTP_PASS", smtp.PasswordRef.Name, smtp.PasswordRef.Key),
		helper.EnvVar("GOTRUE_SMTP_SENDER_NAME", smtp.SenderName),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_INVITE", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_CONFIRMATION", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_RECOVERY", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_EMAIL_CHANGE", "/auth/v1/verify"),
	}

	if smtp.MaxFrequency != nil {
		env = append(env, helper.EnvVar("GOTRUE_SMTP_MAX_FREQUENCY", *smtp.MaxFrequency))
	}

	return env
}

// buildAuthOAuthEnvVars returns the OAuth environment variables for the Auth container.
func buildAuthOAuthEnvVars(project *supabasev1alpha1.Project, apiURL string) []corev1.EnvVar {
	if project.Spec.Auth.OAuth == nil {
		return nil
	}

	env := make([]corev1.EnvVar, 0, 4)
	env = append(env, buildOAuthProviderEnvVars("GOOGLE", project.Spec.Auth.OAuth.Google, apiURL)...)
	env = append(env, buildOAuthProviderEnvVars("GITHUB", project.Spec.Auth.OAuth.GitHub, apiURL)...)
	env = append(env, buildOAuthProviderEnvVars("AZURE", project.Spec.Auth.OAuth.Azure, apiURL)...)
	env = append(env, buildOAuthProviderEnvVars("KEYCLOAK", project.Spec.Auth.OAuth.Keycloak, apiURL)...)
	return env
}

// buildOAuthProviderEnvVars returns the environment variables for a single OAuth provider.
func buildOAuthProviderEnvVars(provider string, config *supabasev1alpha1.OAuthProviderConfig, apiURL string) []corev1.EnvVar {
	if config == nil {
		return nil
	}

	prefix := fmt.Sprintf("GOTRUE_EXTERNAL_%s", provider)
	env := []corev1.EnvVar{
		helper.EnvVar(fmt.Sprintf("%s_ENABLED", prefix), strconv.FormatBool(*config.Enable)),
		helper.EnvVar(fmt.Sprintf("%s_CLIENT_ID", prefix), config.ClientID),
		helper.EnvVarFromSecret(fmt.Sprintf("%s_SECRET", prefix), config.SecretRef.Name, config.SecretRef.Key),
		helper.EnvVar(fmt.Sprintf("%s_REDIRECT_URI", prefix), fmt.Sprintf("%s/auth/v1/callback", apiURL)),
	}

	if config.URL != nil {
		env = append(env, helper.EnvVar(fmt.Sprintf("%s_URL", prefix), *config.URL))
	}

	return env
}

// buildAuthSMSEnvVars returns the SMS environment variables for the Auth container.
func buildAuthSMSEnvVars(project *supabasev1alpha1.Project) []corev1.EnvVar {
	if project.Spec.Auth.SMS == nil {
		return nil
	}

	sms := project.Spec.Auth.SMS
	env := []corev1.EnvVar{
		helper.EnvVar("GOTRUE_SMS_PROVIDER", sms.Provider),
		helper.EnvVar("GOTRUE_SMS_OTP_EXP", strconv.Itoa(int(sms.OTPExp))),
		helper.EnvVar("GOTRUE_SMS_OTP_LENGTH", strconv.Itoa(int(sms.OTPLength))),
		helper.EnvVar("GOTRUE_SMS_MAX_FREQUENCY", sms.MaxFrequency),
		helper.EnvVar("GOTRUE_SMS_TEMPLATE", sms.Template),
	}

	if sms.Twilio != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SMS_TWILIO_ACCOUNT_SID", sms.Twilio.AccountSID),
			helper.EnvVarFromSecret("GOTRUE_SMS_TWILIO_AUTH_TOKEN", sms.Twilio.AuthTokenRef.Name, sms.Twilio.AuthTokenRef.Key),
			helper.EnvVar("GOTRUE_SMS_TWILIO_MESSAGE_SERVICE_SID", sms.Twilio.MessageServiceSID),
		)
	}

	return env
}

// buildAuthMFAEnvVars returns the MFA environment variables for the Auth container.
func buildAuthMFAEnvVars(project *supabasev1alpha1.Project) []corev1.EnvVar {
	if project.Spec.Auth.MFA == nil {
		return nil
	}

	mfa := project.Spec.Auth.MFA
	var env []corev1.EnvVar

	if mfa.EnableTOTPEnroll != nil {
		env = append(env, helper.EnvVar("GOTRUE_MFA_TOTP_ENROLL_ENABLED", strconv.FormatBool(*mfa.EnableTOTPEnroll)))
	}
	if mfa.EnableTOTPVerify != nil {
		env = append(env, helper.EnvVar("GOTRUE_MFA_TOTP_VERIFY_ENABLED", strconv.FormatBool(*mfa.EnableTOTPVerify)))
	}
	if mfa.EnablePhoneEnroll != nil {
		env = append(env, helper.EnvVar("GOTRUE_MFA_PHONE_ENROLL_ENABLED", strconv.FormatBool(*mfa.EnablePhoneEnroll)))
	}
	if mfa.EnablePhoneVerify != nil {
		env = append(env, helper.EnvVar("GOTRUE_MFA_PHONE_VERIFY_ENABLED", strconv.FormatBool(*mfa.EnablePhoneVerify)))
	}
	if mfa.MaxEnrolledFactors != nil {
		env = append(env, helper.EnvVar("GOTRUE_MFA_MAX_ENROLLED_FACTORS", strconv.Itoa(int(*mfa.MaxEnrolledFactors))))
	}

	return env
}

// buildAuthSAMLEnvVars returns the SAML environment variables for the Auth container.
func buildAuthSAMLEnvVars(project *supabasev1alpha1.Project) []corev1.EnvVar {
	if project.Spec.Auth.SAML == nil || !*project.Spec.Auth.SAML.Enable {
		return nil
	}

	saml := project.Spec.Auth.SAML
	env := []corev1.EnvVar{
		helper.EnvVar("GOTRUE_SAML_ENABLED", strconv.FormatBool(*saml.Enable)),
		helper.EnvVarFromSecret("GOTRUE_SAML_PRIVATE_KEY", AuthSecretName(project), AuthSecretSAMLPrivateKey),
	}

	if saml.AllowEncryptedAssertions != nil {
		env = append(env, helper.EnvVar("GOTRUE_SAML_ALLOW_ENCRYPTED_ASSERTIONS", strconv.FormatBool(*saml.AllowEncryptedAssertions)))
	}
	if saml.RelayStateValidityPeriod != nil {
		env = append(env, helper.EnvVar("GOTRUE_SAML_RELAY_STATE_VALIDITY_PERIOD", *saml.RelayStateValidityPeriod))
	}
	if saml.RateLimitAssertion != nil {
		env = append(env, helper.EnvVar("GOTRUE_SAML_RATE_LIMIT_ASSERTION", strconv.Itoa(int(*saml.RateLimitAssertion))))
	}

	return env
}
