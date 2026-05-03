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

package controller

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func derefString(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}

func derefInt32(p *int32, fallback int32) int32 {
	if p != nil {
		return *p
	}
	return fallback
}

func derefBool(p *bool, fallback bool) bool {
	if p != nil {
		return *p
	}
	return fallback
}

func envVar(name, value string) corev1.EnvVar {
	return corev1.EnvVar{Name: name, Value: value}
}

func envVarFromSecret(name, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			},
		},
	}
}

func nodeDNSHost(host string) string {
	return strings.TrimSuffix(host, ".svc.cluster.local")
}

func jwtSecretName(projectName string) string  { return projectName + "-jwt" }
func keysSecretName(projectName string) string { return projectName + "-keys" }
func storageS3ProtocolSecretName(projectName string) string {
	return projectName + "-storage-s3-protocol"
}

func componentServiceName(projectName, component string) string {
	return projectName + "-" + component
}

func StudioEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	name := project.Name
	spec := &project.Spec
	jwtSecret := jwtSecretName(name)
	keysSecret := keysSecretName(name)

	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")

	envs := []corev1.EnvVar{
		envVar("HOSTNAME", "0.0.0.0"),
		envVar("STUDIO_PG_META_URL", fmt.Sprintf("http://%s-meta:8080", name)),
		envVar("POSTGRES_HOST", spec.Database.Host),
		envVar("POSTGRES_PORT", strconv.Itoa(int(dbPort))),
		envVar("POSTGRES_DB", dbName),
		envVarFromSecret("POSTGRES_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		envVarFromSecret("PG_META_CRYPTO_KEY", keysSecret, "crypto-key"),
	}

	if spec.Rest != nil {
		if spec.Rest.DBSchemas != nil {
			envs = append(envs, envVar("PGRST_DB_SCHEMAS", strings.Join(spec.Rest.DBSchemas, ",")))
		}
		envs = append(envs,
			envVar("PGRST_DB_MAX_ROWS", strconv.Itoa(int(derefInt32(spec.Rest.DBMaxRows, 1000)))),
			envVar("PGRST_DB_EXTRA_SEARCH_PATH", derefString(spec.Rest.DBExtraSearchPath, "public")),
		)
	}

	if spec.Studio != nil {
		envs = append(envs,
			envVar("DEFAULT_ORGANIZATION_NAME", derefString(spec.Studio.DefaultOrganization, "Default Organization")),
			envVar("DEFAULT_PROJECT_NAME", derefString(spec.Studio.DefaultProject, "Default Project")),
		)
	}

	envs = append(envs,
		envVar("SUPABASE_URL", InternalURL(project)),
		envVar("SUPABASE_PUBLIC_URL", PublicURL(project)),
		envVarFromSecret("SUPABASE_ANON_KEY", jwtSecret, "anon-key"),
		envVarFromSecret("SUPABASE_SERVICE_KEY", jwtSecret, "service-key"),
		envVarFromSecret("AUTH_JWT_SECRET", jwtSecret, "jwt-secret"),
		envVar("SNIPPETS_MANAGEMENT_FOLDER", "/var/lib/studio/snippets"),
		envVar("EDGE_FUNCTIONS_MANAGEMENT_FOLDER", "/var/lib/studio/functions"),
	)

	if spec.Studio != nil && spec.Studio.AI != nil && spec.Studio.AI.OpenAIApiKeyRef != nil {
		ref := spec.Studio.AI.OpenAIApiKeyRef
		envs = append(envs, envVarFromSecret("OPENAI_API_KEY", ref.Name, ref.Key))
	}

	return envs
}

// nolint:gocyclo
func AuthEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	name := project.Name
	spec := &project.Spec
	jwtSecret := jwtSecretName(name)

	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")
	jwtExpiry := derefInt32(spec.Global.JWTExpirySeconds, 3600)

	envs := []corev1.EnvVar{
		envVar("GOTRUE_API_HOST", "0.0.0.0"),
		envVar("GOTRUE_API_PORT", "9999"),
		envVar("API_EXTERNAL_URL", PublicURL(project)),
		envVar("GOTRUE_DB_DRIVER", "postgres"),
		envVarFromSecret("DB_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		{Name: "GOTRUE_DB_DATABASE_URL", Value: fmt.Sprintf("postgresql://supabase_auth_admin:$(DB_PASSWORD)@%s:%d/%s?sslmode=disable", spec.Database.Host, dbPort, dbName)},
		envVar("GOTRUE_SITE_URL", spec.Global.SiteURL),
		envVar("GOTRUE_JWT_ADMIN_ROLES", "service_role"),
		envVar("GOTRUE_JWT_AUD", "authenticated"),
		envVar("GOTRUE_JWT_DEFAULT_GROUP_NAME", "authenticated"),
		envVar("GOTRUE_JWT_EXP", strconv.Itoa(int(jwtExpiry))),
		envVarFromSecret("GOTRUE_JWT_SECRET", jwtSecret, "jwt-secret"),
		envVarFromSecret("GOTRUE_JWT_KEYS", jwtSecret, "jwt-keys"),
		envVar("GOTRUE_MAILER_URLPATHS_INVITE", "/auth/v1/verify"),
		envVar("GOTRUE_MAILER_URLPATHS_CONFIRMATION", "/auth/v1/verify"),
		envVar("GOTRUE_MAILER_URLPATHS_RECOVERY", "/auth/v1/verify"),
		envVar("GOTRUE_MAILER_URLPATHS_EMAIL_CHANGE", "/auth/v1/verify"),
	}

	if len(spec.Global.AdditionalRedirectURLs) > 0 {
		envs = append(envs, envVar("GOTRUE_URI_ALLOW_LIST", strings.Join(spec.Global.AdditionalRedirectURLs, ",")))
	}

	if spec.Auth != nil {
		envs = append(envs,
			envVar("GOTRUE_DISABLE_SIGNUP", strconv.FormatBool(derefBool(spec.Auth.DisableSignup, false))),
			envVar("GOTRUE_EXTERNAL_ANONYMOUS_USERS_ENABLED", strconv.FormatBool(derefBool(spec.Auth.EnableAnonymousUsers, false))),
			envVar("GOTRUE_EXTERNAL_SKIP_NONCE_CHECK", strconv.FormatBool(derefBool(spec.Auth.ExternalSkipNonceCheck, false))),
		)
	}

	if spec.Auth != nil && spec.Auth.Email != nil {
		envs = append(envs,
			envVar("GOTRUE_EXTERNAL_EMAIL_ENABLED", strconv.FormatBool(derefBool(spec.Auth.Email.EnableSignup, true))),
			envVar("GOTRUE_MAILER_AUTOCONFIRM", strconv.FormatBool(derefBool(spec.Auth.Email.AutoConfirm, false))),
		)
	} else {
		envs = append(envs, envVar("GOTRUE_EXTERNAL_EMAIL_ENABLED", "true"), envVar("GOTRUE_MAILER_AUTOCONFIRM", "false"))
	}

	if spec.Auth != nil && spec.Auth.Phone != nil {
		envs = append(envs,
			envVar("GOTRUE_EXTERNAL_PHONE_ENABLED", strconv.FormatBool(derefBool(spec.Auth.Phone.EnableSignup, false))),
			envVar("GOTRUE_SMS_AUTOCONFIRM", strconv.FormatBool(derefBool(spec.Auth.Phone.AutoConfirm, false))),
		)
	} else {
		envs = append(envs, envVar("GOTRUE_EXTERNAL_PHONE_ENABLED", "false"), envVar("GOTRUE_SMS_AUTOCONFIRM", "false"))
	}

	if spec.Auth != nil && spec.Auth.Email != nil && spec.Auth.Email.SMTP != nil {
		smtp := spec.Auth.Email.SMTP
		envs = append(envs,
			envVar("GOTRUE_SMTP_ADMIN_EMAIL", smtp.AdminEmail),
			envVar("GOTRUE_SMTP_HOST", smtp.Host),
			envVar("GOTRUE_SMTP_PORT", strconv.Itoa(int(smtp.Port))),
			envVarFromSecret("GOTRUE_SMTP_USER", smtp.UserRef.Name, smtp.UserRef.Key),
			envVarFromSecret("GOTRUE_SMTP_PASS", smtp.PassRef.Name, smtp.PassRef.Key),
		)
		if smtp.SenderName != nil {
			envs = append(envs, envVar("GOTRUE_SMTP_SENDER_NAME", *smtp.SenderName))
		}
	}

	if spec.Auth != nil && spec.Auth.OAuth != nil {
		oauth := spec.Auth.OAuth
		if oauth.Google != nil {
			envs = append(envs, envVar("GOTRUE_EXTERNAL_GOOGLE_ENABLED", strconv.FormatBool(derefBool(oauth.Google.Enabled, false))))
			if oauth.Google.ClientIDRef != nil {
				envs = append(envs, envVarFromSecret("GOTRUE_EXTERNAL_GOOGLE_CLIENT_ID", oauth.Google.ClientIDRef.Name, oauth.Google.ClientIDRef.Key))
			}
			if oauth.Google.ClientSecretRef != nil {
				envs = append(envs, envVarFromSecret("GOTRUE_EXTERNAL_GOOGLE_SECRET", oauth.Google.ClientSecretRef.Name, oauth.Google.ClientSecretRef.Key))
			}
		}
		if oauth.GitHub != nil {
			envs = append(envs, envVar("GOTRUE_EXTERNAL_GITHUB_ENABLED", strconv.FormatBool(derefBool(oauth.GitHub.Enabled, false))))
			if oauth.GitHub.ClientIDRef != nil {
				envs = append(envs, envVarFromSecret("GOTRUE_EXTERNAL_GITHUB_CLIENT_ID", oauth.GitHub.ClientIDRef.Name, oauth.GitHub.ClientIDRef.Key))
			}
			if oauth.GitHub.ClientSecretRef != nil {
				envs = append(envs, envVarFromSecret("GOTRUE_EXTERNAL_GITHUB_SECRET", oauth.GitHub.ClientSecretRef.Name, oauth.GitHub.ClientSecretRef.Key))
			}
		}
		if oauth.Azure != nil {
			envs = append(envs, envVar("GOTRUE_EXTERNAL_AZURE_ENABLED", strconv.FormatBool(derefBool(oauth.Azure.Enabled, false))))
			if oauth.Azure.ClientIDRef != nil {
				envs = append(envs, envVarFromSecret("GOTRUE_EXTERNAL_AZURE_CLIENT_ID", oauth.Azure.ClientIDRef.Name, oauth.Azure.ClientIDRef.Key))
			}
			if oauth.Azure.ClientSecretRef != nil {
				envs = append(envs, envVarFromSecret("GOTRUE_EXTERNAL_AZURE_SECRET", oauth.Azure.ClientSecretRef.Name, oauth.Azure.ClientSecretRef.Key))
			}
		}
	}

	if spec.Auth != nil && spec.Auth.SMS != nil {
		sms := spec.Auth.SMS
		envs = append(envs, envVar("GOTRUE_SMS_PROVIDER", sms.Provider))
		if sms.OTPExpSeconds != nil {
			envs = append(envs, envVar("GOTRUE_SMS_OTP_EXP", strconv.Itoa(int(*sms.OTPExpSeconds))))
		}
		if sms.OTPLength != nil {
			envs = append(envs, envVar("GOTRUE_SMS_OTP_LENGTH", strconv.Itoa(int(*sms.OTPLength))))
		}
		if sms.MaxFrequency != nil {
			envs = append(envs, envVar("GOTRUE_SMS_MAX_FREQUENCY", *sms.MaxFrequency))
		}
		if sms.Template != nil {
			envs = append(envs, envVar("GOTRUE_SMS_TEMPLATE", *sms.Template))
		}
		if sms.Twilio != nil {
			envs = append(envs,
				envVarFromSecret("GOTRUE_SMS_TWILIO_ACCOUNT_SID", sms.Twilio.AccountSIDRef.Name, sms.Twilio.AccountSIDRef.Key),
				envVarFromSecret("GOTRUE_SMS_TWILIO_AUTH_TOKEN", sms.Twilio.AuthTokenRef.Name, sms.Twilio.AuthTokenRef.Key),
				envVarFromSecret("GOTRUE_SMS_TWILIO_MESSAGE_SERVICE_SID", sms.Twilio.MessageServiceSIDRef.Name, sms.Twilio.MessageServiceSIDRef.Key),
			)
		}
	}

	if spec.Auth != nil && spec.Auth.MFA != nil {
		mfa := spec.Auth.MFA
		envs = append(envs,
			envVar("GOTRUE_MFA_TOTP_ENROLL_ENABLED", strconv.FormatBool(derefBool(mfa.TOTPEnrollEnabled, false))),
			envVar("GOTRUE_MFA_TOTP_VERIFY_ENABLED", strconv.FormatBool(derefBool(mfa.TOTPVerifyEnabled, false))),
			envVar("GOTRUE_MFA_PHONE_ENROLL_ENABLED", strconv.FormatBool(derefBool(mfa.PhoneEnrollEnabled, false))),
			envVar("GOTRUE_MFA_PHONE_VERIFY_ENABLED", strconv.FormatBool(derefBool(mfa.PhoneVerifyEnabled, false))),
			envVar("GOTRUE_MFA_MAX_ENROLLED_FACTORS", strconv.Itoa(int(derefInt32(mfa.MaxEnrolledFactors, 10)))),
		)
	}

	if spec.Auth != nil && spec.Auth.SAML != nil {
		saml := spec.Auth.SAML
		envs = append(envs, envVar("GOTRUE_SAML_ENABLED", strconv.FormatBool(derefBool(saml.Enabled, false))))
		if saml.PrivateKeyRef != nil {
			envs = append(envs, envVarFromSecret("GOTRUE_SAML_PRIVATE_KEY", saml.PrivateKeyRef.Name, saml.PrivateKeyRef.Key))
		}
		envs = append(envs,
			envVar("GOTRUE_SAML_ALLOW_ENCRYPTED_ASSERTIONS", strconv.FormatBool(derefBool(saml.AllowEncryptedAssertions, false))),
			envVar("GOTRUE_SAML_RELAY_STATE_VALIDITY_PERIOD", derefString(saml.RelayStateValidityPeriod, "2m0s")),
			envVar("GOTRUE_SAML_RATE_LIMIT_ASSERTION", strconv.Itoa(int(derefInt32(saml.RateLimitAssertion, 15)))),
			envVar("GOTRUE_SAML_EXTERNAL_URL", PublicURL(project)+"/auth/v1"),
		)
	}

	return envs
}

func RestEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	spec := &project.Spec
	jwtSecret := jwtSecretName(project.Name)
	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")
	jwtExpiry := derefInt32(spec.Global.JWTExpirySeconds, 3600)

	envs := []corev1.EnvVar{
		envVarFromSecret("DB_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		{Name: "PGRST_DB_URI", Value: fmt.Sprintf("postgresql://authenticator:$(DB_PASSWORD)@%s:%d/%s?sslmode=disable", spec.Database.Host, dbPort, dbName)},
	}

	if spec.Rest != nil && spec.Rest.DBSchemas != nil {
		envs = append(envs, envVar("PGRST_DB_SCHEMAS", strings.Join(spec.Rest.DBSchemas, ",")))
	}
	if spec.Rest != nil {
		envs = append(envs,
			envVar("PGRST_DB_MAX_ROWS", strconv.Itoa(int(derefInt32(spec.Rest.DBMaxRows, 1000)))),
			envVar("PGRST_DB_EXTRA_SEARCH_PATH", derefString(spec.Rest.DBExtraSearchPath, "public")),
		)
	}

	envs = append(envs,
		envVar("PGRST_DB_ANON_ROLE", "anon"),
		envVarFromSecret("PGRST_JWT_SECRET", jwtSecret, "jwt-jwks"),
		envVar("PGRST_DB_USE_LEGACY_GUCS", "false"),
		envVarFromSecret("PGRST_APP_SETTINGS_JWT_SECRET", jwtSecret, "jwt-secret"),
		envVar("PGRST_APP_SETTINGS_JWT_EXP", strconv.Itoa(int(jwtExpiry))),
	)

	return envs
}

func RealtimeEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	spec := &project.Spec
	jwtSecret := jwtSecretName(project.Name)
	keysSecret := keysSecretName(project.Name)
	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")

	return []corev1.EnvVar{
		envVar("PORT", "4000"),
		envVar("DB_HOST", spec.Database.Host),
		envVar("DB_PORT", strconv.Itoa(int(dbPort))),
		envVar("DB_NAME", dbName),
		envVar("DB_USER", "supabase_admin"),
		envVarFromSecret("DB_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		envVar("DB_AFTER_CONNECT_QUERY", "SET search_path TO _realtime"),
		envVar("DB_ENC_KEY", "supabaserealtime"),
		envVarFromSecret("API_JWT_SECRET", jwtSecret, "jwt-secret"),
		envVarFromSecret("METRICS_JWT_SECRET", jwtSecret, "jwt-secret"),
		envVarFromSecret("API_JWT_JWKS", jwtSecret, "jwt-jwks"),
		envVarFromSecret("SECRET_KEY_BASE", keysSecret, "secret-key-base"),
		envVar("ERL_AFLAGS", "-proto_dist inet_tcp"),
		envVar("DNS_NODES", ""),
		envVar("RLIMIT_NOFILE", "10000"),
		envVar("APP_NAME", "realtime"),
		envVar("SEED_SELF_HOST", "true"),
		envVar("RUN_JANITOR", "true"),
		envVar("DISABLE_HEALTHCHECK_LOGGING", "true"),
	}
}

func StorageEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	spec := &project.Spec
	jwtSecret := jwtSecretName(project.Name)
	s3Secret := storageS3ProtocolSecretName(project.Name)
	dbHost := nodeDNSHost(spec.Database.Host)
	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")

	backend := "file"
	if spec.Storage != nil {
		backend = derefString(spec.Storage.Backend, "file")
	}

	envs := []corev1.EnvVar{
		envVar("POSTGRES_HOST", dbHost),
		envVar("POSTGRES_PORT", strconv.Itoa(int(dbPort))),
		envVar("POSTGRES_DB", dbName),
		envVarFromSecret("POSTGRES_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		envVarFromSecret("ANON_KEY", jwtSecret, "anon-key"),
		envVarFromSecret("SERVICE_KEY", jwtSecret, "service-key"),
		envVarFromSecret("AUTH_JWT_SECRET", jwtSecret, "jwt-secret"),
		envVarFromSecret("JWT_JWKS", jwtSecret, "jwt-jwks"),
		envVar("POSTGREST_URL", fmt.Sprintf("http://%s-rest:3000", project.Name)),
		{Name: "DATABASE_URL", Value: fmt.Sprintf("postgres://supabase_storage_admin:$(POSTGRES_PASSWORD)@%s:%d/%s", dbHost, dbPort, dbName)},
		envVar("STORAGE_PUBLIC_URL", StoragePublicURL(project)),
		envVar("REQUEST_ALLOW_X_FORWARDED_PATH", "true"),
		envVar("FILE_SIZE_LIMIT", "52428800"),
		envVar("FILE_STORAGE_BACKEND_PATH", "/var/lib/storage"),
		envVar("STORAGE_BACKEND", backend),
	}

	if spec.Storage != nil {
		envs = append(envs,
			envVar("GLOBAL_S3_BUCKET", derefString(spec.Storage.Bucket, "stub")),
			envVar("TENANT_ID", derefString(spec.Storage.TenantID, "stub")),
			envVar("REGION", derefString(spec.Storage.Region, "local")),
		)
	}

	envs = append(envs,
		envVarFromSecret("S3_PROTOCOL_ACCESS_KEY_ID", s3Secret, "access-key-id"),
		envVarFromSecret("S3_PROTOCOL_ACCESS_KEY_SECRET", s3Secret, "secret-access-key"),
	)

	if backend == "s3" && spec.Storage != nil && spec.Storage.S3 != nil {
		s3 := spec.Storage.S3
		envs = append(envs, envVar("GLOBAL_S3_ENDPOINT", s3.Endpoint))
		if s3.Protocol != nil {
			envs = append(envs, envVar("GLOBAL_S3_PROTOCOL", *s3.Protocol))
		}
		if s3.ForcePathStyle != nil {
			envs = append(envs, envVar("GLOBAL_S3_FORCE_PATH_STYLE", strconv.FormatBool(*s3.ForcePathStyle)))
		}
		envs = append(envs,
			envVarFromSecret("AWS_ACCESS_KEY_ID", s3.AccessKeyRef.Name, s3.AccessKeyRef.Key),
			envVarFromSecret("AWS_SECRET_ACCESS_KEY", s3.SecretKeyRef.Name, s3.SecretKeyRef.Key),
		)
	}

	return envs
}

func MetaEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	spec := &project.Spec
	keysSecret := keysSecretName(project.Name)
	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")

	return []corev1.EnvVar{
		envVar("PG_META_PORT", "8080"),
		envVar("PG_META_DB_HOST", spec.Database.Host),
		envVar("PG_META_DB_PORT", strconv.Itoa(int(dbPort))),
		envVar("PG_META_DB_NAME", dbName),
		envVar("PG_META_DB_USER", "supabase_admin"),
		envVarFromSecret("PG_META_DB_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		envVarFromSecret("CRYPTO_KEY", keysSecret, "crypto-key"),
	}
}

func FunctionsEnvVars(project *platformv1alpha1.Project) []corev1.EnvVar {
	spec := &project.Spec
	jwtSecret := jwtSecretName(project.Name)
	dbHost := nodeDNSHost(spec.Database.Host)
	dbPort := derefInt32(spec.Database.Port, 5432)
	dbName := derefString(spec.Database.DBName, "postgres")

	verifyJWT := "false"
	if spec.Functions != nil {
		verifyJWT = strconv.FormatBool(derefBool(spec.Functions.VerifyJWT, false))
	}

	return []corev1.EnvVar{
		envVarFromSecret("JWT_SECRET", jwtSecret, "jwt-secret"),
		envVarFromSecret("SUPABASE_ANON_KEY", jwtSecret, "anon-key"),
		envVarFromSecret("SUPABASE_SERVICE_ROLE_KEY", jwtSecret, "service-key"),
		envVarFromSecret("SUPABASE_PUBLISHABLE_KEYS", jwtSecret, "publishable-keys-json"),
		envVarFromSecret("SUPABASE_SECRET_KEYS", jwtSecret, "secret-keys-json"),
		envVar("SUPABASE_URL", InternalURL(project)),
		envVar("SUPABASE_PUBLIC_URL", PublicURL(project)),
		envVarFromSecret("DB_PASSWORD", spec.Database.PasswordRef.Name, spec.Database.PasswordRef.Key),
		{Name: "SUPABASE_DB_URL", Value: fmt.Sprintf("postgresql://supabase_functions_admin:$(DB_PASSWORD)@%s:%d/%s?sslmode=disable", dbHost, dbPort, dbName)},
		envVar("VERIFY_JWT", verifyJWT),
	}
}
