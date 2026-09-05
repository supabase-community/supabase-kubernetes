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
	"strconv"

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

	template, err := helper.Overlay(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: AuthLabels(project)},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{buildAuthContainer(project, db)}},
	}, project.Spec.Auth.Pod)
	if err != nil {
		return nil, err
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
			Template: template,
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

// buildAuthContainer returns the Auth container specification.
func buildAuthContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "auth",
		Image:           authImage(project),
		ImagePullPolicy: authImagePullPolicy(project),
		Env:             buildAuthEnvVars(project, db),
		Ports:           authPorts(),
		Resources:       corev1.ResourceRequirements{},
		LivenessProbe:   authLivenessProbe(),
		ReadinessProbe:  authReadinessProbe(),
		StartupProbe:    authStartupProbe(),
	}
}

// authImage returns the Auth image from the spec or the default image.
func authImage(project *supabasev1alpha1.Project) string {
	return DefaultAuthImage
}

// authImagePullPolicy returns the Auth image pull policy from the spec or the default.
func authImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
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
		helper.EnvVar("GOTRUE_JWT_ADMIN_ROLES", "service_role"),
		helper.EnvVar("GOTRUE_JWT_AUD", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_DEFAULT_GROUP_NAME", "authenticated"),
		helper.EnvVar("GOTRUE_JWT_EXP", strconv.Itoa(int(*project.Spec.JWTExpSec))),
		helper.EnvVarFromSecret("GOTRUE_JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVarFromSecret("GOTRUE_JWT_KEYS", JWTSecretName(project), JWTSecretKeys),
		helper.EnvVar("GOTRUE_JWT_ISSUER", fmt.Sprintf("%s/auth/v1", apiURL)),
	}

	return helper.MergeEnvVars(env, auth.Config)
}
