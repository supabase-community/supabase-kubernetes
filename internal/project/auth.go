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
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// EnsureAuth reconciles the Auth Deployment and Service for a Project.
func (r *Reconciler) EnsureAuth(ctx context.Context, project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	logger := log.FromContext(ctx)
	auth := project.Spec.Auth
	if auth == nil {
		return nil
	}

	image := r.resolveAuthImage(project)

	if err := r.ensureAuthService(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Auth Service")
		r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureAuthDeployment(ctx, project, db, image); err != nil {
		logger.Error(err, "Failed to ensure Auth Deployment")
		r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Auth deployment reconciled successfully")
	return nil
}

func (r *Reconciler) resolveAuthImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Auth.Image != nil && *project.Spec.Auth.Image != "" {
		return *project.Spec.Auth.Image
	}
	return DefaultAuthImage
}

func authResourceName(project *supabasev1alpha1.Project) string {
	return project.Name + "-auth"
}

func apiExternalURL(project *supabasev1alpha1.Project) string {
	url := fmt.Sprintf("%s://%s", project.Spec.HTTP.Protocol, project.Spec.HTTP.Hostname)
	if project.Spec.HTTP.Port != nil {
		url = fmt.Sprintf("%s:%d", url, *project.Spec.HTTP.Port)
	}
	return url
}

func (r *Reconciler) ensureAuthService(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx).WithValues("service", authResourceName(project))
	auth := project.Spec.Auth

	svcSpec := auth.Service
	if svcSpec == nil {
		svcSpec = &supabasev1alpha1.ServiceSpec{}
	}

	svcType := corev1.ServiceTypeClusterIP
	if svcSpec.Type != nil && *svcSpec.Type != "" {
		svcType = *svcSpec.Type
	}

	port := int32(AuthPort)

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        authResourceName(project),
			Namespace:   project.Namespace,
			Labels:      r.labelsForAuth(project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForAuth(project),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(AuthPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on service: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Client.Get(ctx, namespacedName(desired.Name, desired.Namespace), existing)
	if err != nil {
		if !clientIsNotFound(err) {
			return fmt.Errorf("getting service: %w", err)
		}
		logger.Info("Creating Service")
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating service: %w", err)
		}
		logger.Info("Created Service")
		return nil
	}

	existing.Spec.Type = desired.Spec.Type
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Ports = desired.Spec.Ports
	existing.Annotations = desired.Annotations
	existing.Labels = desired.Labels

	if err := r.Client.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	logger.V(1).Info("Updated Service")
	return nil
}

func (r *Reconciler) ensureAuthDeployment(ctx context.Context, project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", authResourceName(project))
	auth := project.Spec.Auth

	replicas := int32(1)
	if auth.Replicas != nil {
		replicas = *auth.Replicas
	}

	labels := r.labelsForAuth(project)
	selectorLabels := r.selectorLabelsForAuth(project)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, auth.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      authResourceName(project),
			Namespace: project.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: auth.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:        auth.Affinity,
					NodeSelector:    auth.NodeSelector,
					Tolerations:     auth.Tolerations,
					SecurityContext: auth.PodSecurityContext,
					Containers: []corev1.Container{
						r.buildAuthContainer(project, db, image),
					},
				},
			},
		},
	}
	if auth.PriorityClassName != nil {
		desired.Spec.Template.Spec.PriorityClassName = *auth.PriorityClassName
	}

	if auth.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = auth.TerminationGracePeriodSeconds
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on deployment: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, namespacedName(desired.Name, desired.Namespace), existing)
	if err != nil {
		if !clientIsNotFound(err) {
			return fmt.Errorf("getting deployment: %w", err)
		}
		logger.Info("Creating Deployment")
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating deployment: %w", err)
		}
		logger.Info("Created Deployment")
		return nil
	}

	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels

	if err := r.Client.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating deployment: %w", err)
	}
	logger.V(1).Info("Updated Deployment")
	return nil
}

func (r *Reconciler) buildAuthContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, image string) corev1.Container {
	auth := project.Spec.Auth
	resolved := db
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabase{}
	}

	jwtExpiry := "3600"
	if project.Spec.JWTExpSec != nil {
		jwtExpiry = strconv.Itoa(int(*project.Spec.JWTExpSec))
	}

	projectJWTSecret := fmt.Sprintf("%s-jwt", project.Name)
	externalURL := apiExternalURL(project)

	env := []corev1.EnvVar{
		helper.EnvVarFromSecret("GOTRUE_DB_PASSWORD", resolved.PasswordRef.Name, resolved.PasswordRef.Key),
		helper.EnvVarFromSecret("GOTRUE_JWT_SECRET", projectJWTSecret, "jwt-secret"),
		helper.EnvVarFromSecret("GOTRUE_JWT_KEYS", projectJWTSecret, "jwt-keys"),
	}

	env = append(env, r.buildAuthSecretEnv(auth, project)...)
	env = append(env,
		helper.EnvVar("GOTRUE_API_HOST", "0.0.0.0"),
		helper.EnvVar("GOTRUE_API_PORT", "9999"),
		helper.EnvVar("API_EXTERNAL_URL", externalURL),
		helper.EnvVar("GOTRUE_DB_DRIVER", "postgres"),
		helper.EnvVar("GOTRUE_DB_DATABASE_URL", fmt.Sprintf("postgres://supabase_auth_admin:%s@%s:%d/%s",
			"$(GOTRUE_DB_PASSWORD)",
			resolved.Host,
			resolved.Port,
			resolved.DBName,
		)),
		helper.EnvVar("GOTRUE_SITE_URL", auth.SiteURL),
		helper.EnvVar("GOTRUE_DISABLE_SIGNUP", strconv.FormatBool(auth.DisableSignup)),
		helper.EnvVar("GOTRUE_JWT_ADMIN_ROLES", "service_role"),
		helper.EnvVar("GOTRUE_JWT_AUD", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_DEFAULT_GROUP_NAME", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_EXP", jwtExpiry),
		helper.EnvVar("GOTRUE_JWT_ISSUER", fmt.Sprintf("%s/auth/v1", externalURL)),
		helper.EnvVar("GOTRUE_EXTERNAL_EMAIL_ENABLED", strconv.FormatBool(auth.EnableEmailSignup)),
		helper.EnvVar("GOTRUE_EXTERNAL_ANONYMOUS_USERS_ENABLED", strconv.FormatBool(auth.EnableAnonymousUsers)),
		helper.EnvVar("GOTRUE_MAILER_AUTOCONFIRM", strconv.FormatBool(auth.EnableEmailAutoconfirm)),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_INVITE", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_CONFIRMATION", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_RECOVERY", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_EMAIL_CHANGE", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_EXTERNAL_PHONE_ENABLED", strconv.FormatBool(auth.EnablePhoneSignup)),
		helper.EnvVar("GOTRUE_SMS_AUTOCONFIRM", strconv.FormatBool(auth.EnablePhoneAutoconfirm)),
	)

	if len(auth.AdditionalRedirectURLs) > 0 {
		env = append(env, helper.EnvVar("GOTRUE_URI_ALLOW_LIST", strings.Join(auth.AdditionalRedirectURLs, ",")))
	}

	if auth.SkipNonceCheck != nil {
		env = append(env, helper.EnvVar("GOTRUE_EXTERNAL_SKIP_NONCE_CHECK", strconv.FormatBool(*auth.SkipNonceCheck)))
	}

	if auth.MailerSecureEmailChangeEnabled != nil {
		env = append(env, helper.EnvVar("GOTRUE_MAILER_SECURE_EMAIL_CHANGE_ENABLED", strconv.FormatBool(*auth.MailerSecureEmailChangeEnabled)))
	}

	env = append(env, r.buildAuthSMTPMFAEnv(auth, externalURL)...)

	container := corev1.Container{
		Name:  "auth",
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: AuthPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:             env,
		SecurityContext: auth.ContainerSecurityContext,
	}
	if auth.ImagePullPolicy != nil {
		container.ImagePullPolicy = *auth.ImagePullPolicy
	}
	if auth.Resources != nil {
		container.Resources = *auth.Resources
	}

	return container
}

func (r *Reconciler) buildAuthSecretEnv(auth *supabasev1alpha1.AuthSpec, project *supabasev1alpha1.Project) []corev1.EnvVar {
	env := []corev1.EnvVar{}

	if auth.SMTP != nil {
		env = append(env, helper.EnvVarFromSecret("GOTRUE_SMTP_PASS",
			auth.SMTP.PasswordRef.Name, auth.SMTP.PasswordRef.Key))
	}

	if auth.SAML != nil && auth.SAML.Enabled {
		env = append(env, helper.EnvVarFromSecret("GOTRUE_SAML_PRIVATE_KEY",
			fmt.Sprintf("%s-keys", project.Name), "saml-private-key"))
	}

	if auth.OAuth != nil {
		if auth.OAuth.Google != nil {
			env = append(env, helper.EnvVarFromSecret("GOTRUE_EXTERNAL_GOOGLE_SECRET",
				auth.OAuth.Google.SecretRef.Name, auth.OAuth.Google.SecretRef.Key))
		}
		if auth.OAuth.GitHub != nil {
			env = append(env, helper.EnvVarFromSecret("GOTRUE_EXTERNAL_GITHUB_SECRET",
				auth.OAuth.GitHub.SecretRef.Name, auth.OAuth.GitHub.SecretRef.Key))
		}
		if auth.OAuth.Azure != nil {
			env = append(env, helper.EnvVarFromSecret("GOTRUE_EXTERNAL_AZURE_SECRET",
				auth.OAuth.Azure.SecretRef.Name, auth.OAuth.Azure.SecretRef.Key))
		}
	}

	if auth.SMS != nil {
		env = append(env, helper.EnvVarFromSecret("GOTRUE_SMS_TWILIO_AUTH_TOKEN",
			auth.SMS.TwilioAuthTokenRef.Name, auth.SMS.TwilioAuthTokenRef.Key))
	}

	return env
}

func (r *Reconciler) buildAuthSMTPMFAEnv(auth *supabasev1alpha1.AuthSpec, externalURL string) []corev1.EnvVar {
	env := []corev1.EnvVar{}

	if auth.SMTP != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SMTP_HOST", auth.SMTP.Host),
			helper.EnvVar("GOTRUE_SMTP_PORT", strconv.Itoa(int(auth.SMTP.Port))),
			helper.EnvVar("GOTRUE_SMTP_USER", auth.SMTP.User),
			helper.EnvVar("GOTRUE_SMTP_ADMIN_EMAIL", auth.SMTP.AdminEmail),
			helper.EnvVar("GOTRUE_SMTP_SENDER_NAME", auth.SMTP.SenderName),
		)
		if auth.SMTP.MaxFrequency != nil && *auth.SMTP.MaxFrequency != "" {
			env = append(env, helper.EnvVar("GOTRUE_SMTP_MAX_FREQUENCY", *auth.SMTP.MaxFrequency))
		}
	}

	if auth.OAuth != nil {
		if auth.OAuth.Google != nil {
			env = append(env,
				helper.EnvVar("GOTRUE_EXTERNAL_GOOGLE_ENABLED", strconv.FormatBool(auth.OAuth.Google.Enabled)),
				helper.EnvVar("GOTRUE_EXTERNAL_GOOGLE_CLIENT_ID", auth.OAuth.Google.ClientID),
				helper.EnvVar("GOTRUE_EXTERNAL_GOOGLE_REDIRECT_URI", fmt.Sprintf("%s/auth/v1/callback", externalURL)),
			)
		}
		if auth.OAuth.GitHub != nil {
			env = append(env,
				helper.EnvVar("GOTRUE_EXTERNAL_GITHUB_ENABLED", strconv.FormatBool(auth.OAuth.GitHub.Enabled)),
				helper.EnvVar("GOTRUE_EXTERNAL_GITHUB_CLIENT_ID", auth.OAuth.GitHub.ClientID),
				helper.EnvVar("GOTRUE_EXTERNAL_GITHUB_REDIRECT_URI", fmt.Sprintf("%s/auth/v1/callback", externalURL)),
			)
		}
		if auth.OAuth.Azure != nil {
			env = append(env,
				helper.EnvVar("GOTRUE_EXTERNAL_AZURE_ENABLED", strconv.FormatBool(auth.OAuth.Azure.Enabled)),
				helper.EnvVar("GOTRUE_EXTERNAL_AZURE_CLIENT_ID", auth.OAuth.Azure.ClientID),
				helper.EnvVar("GOTRUE_EXTERNAL_AZURE_REDIRECT_URI", fmt.Sprintf("%s/auth/v1/callback", externalURL)),
			)
		}
	}

	if auth.SMS != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SMS_PROVIDER", auth.SMS.Provider),
			helper.EnvVar("GOTRUE_SMS_OTP_EXP", strconv.Itoa(int(auth.SMS.OTPExp))),
			helper.EnvVar("GOTRUE_SMS_OTP_LENGTH", strconv.Itoa(int(auth.SMS.OTPLength))),
			helper.EnvVar("GOTRUE_SMS_TEMPLATE", auth.SMS.Template),
			helper.EnvVar("GOTRUE_SMS_TWILIO_ACCOUNT_SID", auth.SMS.TwilioAccountSID),
			helper.EnvVar("GOTRUE_SMS_TWILIO_MESSAGE_SERVICE_SID", auth.SMS.TwilioMessageServiceSID),
			helper.EnvVar("GOTRUE_SMS_MAX_FREQUENCY", auth.SMS.MaxFrequency),
		)
	}

	if auth.MFA != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_MFA_TOTP_ENROLL_ENABLED", strconv.FormatBool(auth.MFA.TOTPEnrollEnabled != nil && *auth.MFA.TOTPEnrollEnabled)),
			helper.EnvVar("GOTRUE_MFA_TOTP_VERIFY_ENABLED", strconv.FormatBool(auth.MFA.TOTPVerifyEnabled != nil && *auth.MFA.TOTPVerifyEnabled)),
			helper.EnvVar("GOTRUE_MFA_PHONE_ENROLL_ENABLED", strconv.FormatBool(auth.MFA.PhoneEnrollEnabled != nil && *auth.MFA.PhoneEnrollEnabled)),
			helper.EnvVar("GOTRUE_MFA_PHONE_VERIFY_ENABLED", strconv.FormatBool(auth.MFA.PhoneVerifyEnabled != nil && *auth.MFA.PhoneVerifyEnabled)),
		)
		if auth.MFA.MaxEnrolledFactors != nil {
			env = append(env, helper.EnvVar("GOTRUE_MFA_MAX_ENROLLED_FACTORS", strconv.Itoa(int(*auth.MFA.MaxEnrolledFactors))))
		}
	}

	if auth.SAML != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SAML_ENABLED", strconv.FormatBool(auth.SAML.Enabled)),
			helper.EnvVar("GOTRUE_SAML_ALLOW_ENCRYPTED_ASSERTIONS", strconv.FormatBool(auth.SAML.AllowEncryptedAssertions != nil && *auth.SAML.AllowEncryptedAssertions)),
		)
		if auth.SAML.RelayStateValidityPeriod != nil && *auth.SAML.RelayStateValidityPeriod != "" {
			env = append(env, helper.EnvVar("GOTRUE_SAML_RELAY_STATE_VALIDITY_PERIOD", *auth.SAML.RelayStateValidityPeriod))
		}
		if auth.SAML.RateLimitAssertion != nil {
			env = append(env, helper.EnvVar("GOTRUE_SAML_RATE_LIMIT_ASSERTIONS", strconv.Itoa(int(*auth.SAML.RateLimitAssertion))))
		}
	}

	return env
}

func (r *Reconciler) labelsForAuth(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "auth",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/component":  "auth",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *Reconciler) selectorLabelsForAuth(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "auth",
		"app.kubernetes.io/instance": project.Name,
	}
}
