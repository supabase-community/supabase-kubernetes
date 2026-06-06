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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	"github.com/supabase-community/supabase-kubernetes/internal/images"
)

// EnsureAuth reconciles the Auth Deployment and Service for a Project.
func (r *Reconciler) EnsureAuth(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx)
	ref := project.Spec.AuthRef
	if ref == nil {
		return nil
	}

	auth := &supabasev1alpha1.Auth{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: project.Namespace}, auth); err != nil {
		if apierrors.IsNotFound(err) {
			r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionFalse, "ComponentNotFound",
				fmt.Sprintf("Auth %q not found", ref.Name))
			logger.Error(err, "Auth resource not found", "auth", ref.Name)
			return fmt.Errorf("auth %q not found", ref.Name)
		}
		logger.Error(err, "Failed to get Auth", "auth", ref.Name)
		return err
	}

	image, err := r.resolveAuthImage(auth, project)
	if err != nil {
		r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionFalse, "VersionNotSupported", err.Error())
		return err
	}

	if err := r.ensureAuthService(ctx, project, auth); err != nil {
		logger.Error(err, "Failed to ensure Auth Service")
		r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureAuthDeployment(ctx, project, auth, image); err != nil {
		logger.Error(err, "Failed to ensure Auth Deployment")
		r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeAuthReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Auth deployment reconciled successfully")
	return nil
}

func (r *Reconciler) resolveAuthImage(auth *supabasev1alpha1.Auth, project *supabasev1alpha1.Project) (string, error) {
	if auth.Spec.Image != "" {
		return auth.Spec.Image, nil
	}
	return images.Resolve(project.Spec.Version, images.ComponentAuth)
}

func authResourceName(auth *supabasev1alpha1.Auth) string {
	return auth.Name + "-auth"
}

func apiExternalURL(project *supabasev1alpha1.Project) string {
	url := fmt.Sprintf("%s://%s", project.Spec.HTTP.Protocol, project.Spec.HTTP.Hostname)
	if project.Spec.HTTP.Port != nil {
		url = fmt.Sprintf("%s:%d", url, *project.Spec.HTTP.Port)
	}
	return url
}

func (r *Reconciler) ensureAuthService(ctx context.Context, project *supabasev1alpha1.Project, auth *supabasev1alpha1.Auth) error {
	logger := log.FromContext(ctx).WithValues("service", authResourceName(auth))

	svcSpec := auth.Spec.Service
	if svcSpec == nil {
		svcSpec = &supabasev1alpha1.ServiceSpec{}
	}

	svcType := corev1.ServiceTypeClusterIP
	if svcSpec.Type != "" {
		svcType = svcSpec.Type
	}

	port := int32(AuthPort)

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        authResourceName(auth),
			Namespace:   auth.Namespace,
			Labels:      r.labelsForAuth(auth, project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForAuth(auth),
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
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
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

func (r *Reconciler) ensureAuthDeployment(ctx context.Context, project *supabasev1alpha1.Project, auth *supabasev1alpha1.Auth, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", authResourceName(auth))

	replicas := int32(1)
	if auth.Spec.Replicas != nil {
		replicas = *auth.Spec.Replicas
	}

	labels := r.labelsForAuth(auth, project)
	selectorLabels := r.selectorLabelsForAuth(auth)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, auth.Spec.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      authResourceName(auth),
			Namespace: auth.Namespace,
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
					Annotations: auth.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:          auth.Spec.Affinity,
					NodeSelector:      auth.Spec.NodeSelector,
					Tolerations:       auth.Spec.Tolerations,
					PriorityClassName: auth.Spec.PriorityClassName,
					SecurityContext:   auth.Spec.PodSecurityContext,
					Containers: []corev1.Container{
						r.buildAuthContainer(auth, project, image),
					},
				},
			},
		},
	}

	if auth.Spec.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = auth.Spec.TerminationGracePeriodSeconds
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on deployment: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
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

func (r *Reconciler) buildAuthContainer(auth *supabasev1alpha1.Auth, project *supabasev1alpha1.Project, image string) corev1.Container {
	resolved := project.Status.ResolvedDatabase
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabase{}
	}

	jwtExpiry := "3600"
	if project.Spec.JWTExpirySeconds != nil {
		jwtExpiry = strconv.Itoa(int(*project.Spec.JWTExpirySeconds))
	}

	projectJWTSecret := fmt.Sprintf("%s-jwt", project.Name)
	externalURL := apiExternalURL(project)

	env := []corev1.EnvVar{
		helper.EnvVarFromSecret("GOTRUE_DB_PASSWORD", resolved.PasswordRef.Name, resolved.PasswordRef.Key),
		helper.EnvVarFromSecret("GOTRUE_JWT_SECRET", projectJWTSecret, "jwt-secret"),
		helper.EnvVarFromSecret("GOTRUE_JWT_KEYS", projectJWTSecret, "jwt-keys"),
	}

	if auth.Spec.SMTP != nil {
		env = append(env, helper.EnvVarFromSecret("GOTRUE_SMTP_PASS",
			auth.Spec.SMTP.PasswordRef.Name, auth.Spec.SMTP.PasswordRef.Key))
	}

	if auth.Spec.SAML != nil && auth.Spec.SAML.Enabled {
		env = append(env, helper.EnvVarFromSecret("GOTRUE_SAML_PRIVATE_KEY",
			fmt.Sprintf("%s-keys", project.Name), "saml-private-key"))
	}

	if auth.Spec.OAuth != nil {
		if auth.Spec.OAuth.Google != nil {
			env = append(env, helper.EnvVarFromSecret("GOTRUE_EXTERNAL_GOOGLE_SECRET",
				auth.Spec.OAuth.Google.SecretRef.Name, auth.Spec.OAuth.Google.SecretRef.Key))
		}
		if auth.Spec.OAuth.GitHub != nil {
			env = append(env, helper.EnvVarFromSecret("GOTRUE_EXTERNAL_GITHUB_SECRET",
				auth.Spec.OAuth.GitHub.SecretRef.Name, auth.Spec.OAuth.GitHub.SecretRef.Key))
		}
		if auth.Spec.OAuth.Azure != nil {
			env = append(env, helper.EnvVarFromSecret("GOTRUE_EXTERNAL_AZURE_SECRET",
				auth.Spec.OAuth.Azure.SecretRef.Name, auth.Spec.OAuth.Azure.SecretRef.Key))
		}
	}

	if auth.Spec.SMS != nil {
		env = append(env, helper.EnvVarFromSecret("GOTRUE_SMS_TWILIO_AUTH_TOKEN",
			auth.Spec.SMS.TwilioAuthTokenRef.Name, auth.Spec.SMS.TwilioAuthTokenRef.Key))
	}

	env = append(env, auth.Spec.Env...)

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
		helper.EnvVar("GOTRUE_SITE_URL", auth.Spec.SiteURL),
		helper.EnvVar("GOTRUE_DISABLE_SIGNUP", strconv.FormatBool(auth.Spec.DisableSignup)),
		helper.EnvVar("GOTRUE_JWT_ADMIN_ROLES", "service_role"),
		helper.EnvVar("GOTRUE_JWT_AUD", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_DEFAULT_GROUP_NAME", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_EXP", jwtExpiry),
		helper.EnvVar("GOTRUE_JWT_ISSUER", fmt.Sprintf("%s/auth/v1", externalURL)),
		helper.EnvVar("GOTRUE_EXTERNAL_EMAIL_ENABLED", strconv.FormatBool(auth.Spec.EnableEmailSignup)),
		helper.EnvVar("GOTRUE_EXTERNAL_ANONYMOUS_USERS_ENABLED", strconv.FormatBool(auth.Spec.EnableAnonymousUsers)),
		helper.EnvVar("GOTRUE_MAILER_AUTOCONFIRM", strconv.FormatBool(auth.Spec.EnableEmailAutoconfirm)),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_INVITE", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_CONFIRMATION", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_RECOVERY", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_MAILER_URLPATHS_EMAIL_CHANGE", "/auth/v1/verify"),
		helper.EnvVar("GOTRUE_EXTERNAL_PHONE_ENABLED", strconv.FormatBool(auth.Spec.EnablePhoneSignup)),
		helper.EnvVar("GOTRUE_SMS_AUTOCONFIRM", strconv.FormatBool(auth.Spec.EnablePhoneAutoconfirm)),
	)

	if len(auth.Spec.AdditionalRedirectURLs) > 0 {
		env = append(env, helper.EnvVar("GOTRUE_URI_ALLOW_LIST", strings.Join(auth.Spec.AdditionalRedirectURLs, ",")))
	}

	if auth.Spec.SkipNonceCheck != nil {
		env = append(env, helper.EnvVar("GOTRUE_EXTERNAL_SKIP_NONCE_CHECK", strconv.FormatBool(*auth.Spec.SkipNonceCheck)))
	}

	if auth.Spec.MailerSecureEmailChangeEnabled != nil {
		env = append(env, helper.EnvVar("GOTRUE_MAILER_SECURE_EMAIL_CHANGE_ENABLED", strconv.FormatBool(*auth.Spec.MailerSecureEmailChangeEnabled)))
	}

	if auth.Spec.SMTP != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SMTP_HOST", auth.Spec.SMTP.Host),
			helper.EnvVar("GOTRUE_SMTP_PORT", strconv.Itoa(int(auth.Spec.SMTP.Port))),
			helper.EnvVar("GOTRUE_SMTP_USER", auth.Spec.SMTP.User),
			helper.EnvVar("GOTRUE_SMTP_ADMIN_EMAIL", auth.Spec.SMTP.AdminEmail),
			helper.EnvVar("GOTRUE_SMTP_SENDER_NAME", auth.Spec.SMTP.SenderName),
		)
		if auth.Spec.SMTP.MaxFrequency != "" {
			env = append(env, helper.EnvVar("GOTRUE_SMTP_MAX_FREQUENCY", auth.Spec.SMTP.MaxFrequency))
		}
	}

	if auth.Spec.OAuth != nil {
		if auth.Spec.OAuth.Google != nil {
			env = append(env,
				helper.EnvVar("GOTRUE_EXTERNAL_GOOGLE_ENABLED", strconv.FormatBool(auth.Spec.OAuth.Google.Enabled)),
				helper.EnvVar("GOTRUE_EXTERNAL_GOOGLE_CLIENT_ID", auth.Spec.OAuth.Google.ClientID),
				helper.EnvVar("GOTRUE_EXTERNAL_GOOGLE_REDIRECT_URI", fmt.Sprintf("%s/auth/v1/callback", externalURL)),
			)
		}
		if auth.Spec.OAuth.GitHub != nil {
			env = append(env,
				helper.EnvVar("GOTRUE_EXTERNAL_GITHUB_ENABLED", strconv.FormatBool(auth.Spec.OAuth.GitHub.Enabled)),
				helper.EnvVar("GOTRUE_EXTERNAL_GITHUB_CLIENT_ID", auth.Spec.OAuth.GitHub.ClientID),
				helper.EnvVar("GOTRUE_EXTERNAL_GITHUB_REDIRECT_URI", fmt.Sprintf("%s/auth/v1/callback", externalURL)),
			)
		}
		if auth.Spec.OAuth.Azure != nil {
			env = append(env,
				helper.EnvVar("GOTRUE_EXTERNAL_AZURE_ENABLED", strconv.FormatBool(auth.Spec.OAuth.Azure.Enabled)),
				helper.EnvVar("GOTRUE_EXTERNAL_AZURE_CLIENT_ID", auth.Spec.OAuth.Azure.ClientID),
				helper.EnvVar("GOTRUE_EXTERNAL_AZURE_REDIRECT_URI", fmt.Sprintf("%s/auth/v1/callback", externalURL)),
			)
		}
	}

	if auth.Spec.SMS != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SMS_PROVIDER", auth.Spec.SMS.Provider),
			helper.EnvVar("GOTRUE_SMS_OTP_EXP", strconv.Itoa(int(auth.Spec.SMS.OTPExp))),
			helper.EnvVar("GOTRUE_SMS_OTP_LENGTH", strconv.Itoa(int(auth.Spec.SMS.OTPLength))),
			helper.EnvVar("GOTRUE_SMS_TEMPLATE", auth.Spec.SMS.Template),
			helper.EnvVar("GOTRUE_SMS_TWILIO_ACCOUNT_SID", auth.Spec.SMS.TwilioAccountSID),
			helper.EnvVar("GOTRUE_SMS_TWILIO_MESSAGE_SERVICE_SID", auth.Spec.SMS.TwilioMessageServiceSID),
			helper.EnvVar("GOTRUE_SMS_MAX_FREQUENCY", auth.Spec.SMS.MaxFrequency),
		)
	}

	if auth.Spec.MFA != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_MFA_TOTP_ENROLL_ENABLED", strconv.FormatBool(auth.Spec.MFA.TOTPEnrollEnabled)),
			helper.EnvVar("GOTRUE_MFA_TOTP_VERIFY_ENABLED", strconv.FormatBool(auth.Spec.MFA.TOTPVerifyEnabled)),
			helper.EnvVar("GOTRUE_MFA_PHONE_ENROLL_ENABLED", strconv.FormatBool(auth.Spec.MFA.PhoneEnrollEnabled)),
			helper.EnvVar("GOTRUE_MFA_PHONE_VERIFY_ENABLED", strconv.FormatBool(auth.Spec.MFA.PhoneVerifyEnabled)),
		)
		if auth.Spec.MFA.MaxEnrolledFactors > 0 {
			env = append(env, helper.EnvVar("GOTRUE_MFA_MAX_ENROLLED_FACTORS", strconv.Itoa(int(auth.Spec.MFA.MaxEnrolledFactors))))
		}
	}

	if auth.Spec.SAML != nil {
		env = append(env,
			helper.EnvVar("GOTRUE_SAML_ENABLED", strconv.FormatBool(auth.Spec.SAML.Enabled)),
			helper.EnvVar("GOTRUE_SAML_ALLOW_ENCRYPTED_ASSERTIONS", strconv.FormatBool(auth.Spec.SAML.AllowEncryptedAssertions)),
		)
		if auth.Spec.SAML.RelayStateValidityPeriod != "" {
			env = append(env, helper.EnvVar("GOTRUE_SAML_RELAY_STATE_VALIDITY_PERIOD", auth.Spec.SAML.RelayStateValidityPeriod))
		}
		if auth.Spec.SAML.RateLimitAssertion > 0 {
			env = append(env, helper.EnvVar("GOTRUE_SAML_RATE_LIMIT_ASSERTIONS", strconv.Itoa(int(auth.Spec.SAML.RateLimitAssertion))))
		}
	}

	container := corev1.Container{
		Name:            "auth",
		Image:           image,
		ImagePullPolicy: auth.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: AuthPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:             env,
		Resources:       auth.Spec.Resources,
		SecurityContext: auth.Spec.ContainerSecurityContext,
	}

	if auth.Spec.Probes != nil {
		if auth.Spec.Probes.Startup != nil {
			container.StartupProbe = auth.Spec.Probes.Startup
		}
		if auth.Spec.Probes.Readiness != nil {
			container.ReadinessProbe = auth.Spec.Probes.Readiness
		}
		if auth.Spec.Probes.Liveness != nil {
			container.LivenessProbe = auth.Spec.Probes.Liveness
		}
	}

	return container
}

func (r *Reconciler) labelsForAuth(auth *supabasev1alpha1.Auth, project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "auth",
		"app.kubernetes.io/instance":   auth.Name,
		"app.kubernetes.io/component":  "auth",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *Reconciler) selectorLabelsForAuth(auth *supabasev1alpha1.Auth) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "auth",
		"app.kubernetes.io/instance": auth.Name,
	}
}
