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

// RealtimeDeploymentName returns the name of the Realtime Deployment for a Project.
func RealtimeDeploymentName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-realtime", project.Name)
}

// RealtimeDeployment constructs the Realtime Deployment for a Project.
func RealtimeDeployment(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.Deployment, error) {
	if project.Spec.Realtime == nil || !*project.Spec.Realtime.Enable {
		return nil, nil
	}

	template, err := helper.Overlay(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: RealtimeLabels(project)},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{buildRealtimeContainer(project, db)}},
	}, project.Spec.Realtime.Pod)
	if err != nil {
		return nil, err
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RealtimeDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    RealtimeLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: realtimeReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: RealtimeSelectorLabels(project),
			},
			Template: template,
		},
	}

	return deploy, nil
}

// realtimeReplicas returns the number of Realtime replicas from the spec or the default.
func realtimeReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Realtime != nil && project.Spec.Realtime.Replicas != nil {
		return project.Spec.Realtime.Replicas
	}
	return ptr.To(int32(1))
}

// buildRealtimeContainer returns the Realtime container specification.
func buildRealtimeContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "realtime",
		Image:           realtimeImage(project),
		ImagePullPolicy: realtimeImagePullPolicy(project),
		Env:             buildRealtimeEnvVars(project, db),
		Ports:           realtimePorts(),
		Resources:       corev1.ResourceRequirements{},
		LivenessProbe:   realtimeLivenessProbe(),
		ReadinessProbe:  realtimeReadinessProbe(),
		StartupProbe:    realtimeStartupProbe(),
	}
}

// realtimeImage returns the Realtime image from the spec or the default image.
func realtimeImage(project *supabasev1alpha1.Project) string {
	return DefaultRealtimeImage
}

// realtimeImagePullPolicy returns the Realtime image pull policy from the spec or the default.
func realtimeImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	return corev1.PullIfNotPresent
}

// realtimePorts returns the container ports for the Realtime container.
func realtimePorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "realtime",
			ContainerPort: DefaultRealtimePort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// realtimeLivenessProbe returns the liveness probe for the Realtime container.
func realtimeLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        realtimeProbeHandler(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// realtimeReadinessProbe returns the readiness probe for the Realtime container.
func realtimeReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        realtimeProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// realtimeStartupProbe returns the startup probe for the Realtime container.
func realtimeStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        realtimeProbeHandler(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// realtimeProbeHandler returns the shared probe handler for Realtime health checks.
func realtimeProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"/bin/sh",
				"-c",
				fmt.Sprintf(
					"curl -sSfL --head -o /dev/null -H \"Authorization: Bearer ${ANON_KEY}\" http://localhost:%s/api/tenants/realtime-dev/health",
					strconv.Itoa(int(DefaultRealtimePort)),
				),
			},
		},
	}
}

// buildRealtimeEnvVars returns the environment variables for the Realtime container.
func buildRealtimeEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	port := strconv.Itoa(int(DefaultRealtimePort))

	return []corev1.EnvVar{
		helper.EnvVar("PORT", port),
		helper.EnvVar("DB_HOST", db.Host),
		helper.EnvVar("DB_PORT", strconv.Itoa(int(db.Port))),
		helper.EnvVar("DB_USER", "supabase_admin"),
		helper.EnvVarFromSecret("DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("DB_NAME", db.DBName),
		helper.EnvVar("DB_AFTER_CONNECT_QUERY", "SET search_path TO _realtime"),
		helper.EnvVarFromSecret("DB_ENC_KEY", KeysSecretName(project), KeysSecretRealtimeDBEncKey),
		helper.EnvVarFromSecret("API_JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVarFromSecret("API_JWT_JWKS", JWTSecretName(project), JWTSecretJWKS),
		helper.EnvVarFromSecret("SECRET_KEY_BASE", KeysSecretName(project), KeysSecretSecretKeyBase),
		helper.EnvVarFromSecret("METRICS_JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVar("ERL_AFLAGS", "-proto_dist inet_tcp"),
		helper.EnvVar("DNS_NODES", "''"),
		helper.EnvVar("RLIMIT_NOFILE", "10000"),
		helper.EnvVar("APP_NAME", "realtime"),
		helper.EnvVar("SEED_SELF_HOST", "true"),
		helper.EnvVar("RUN_JANITOR", "true"),
		helper.EnvVar("DISABLE_HEALTHCHECK_LOGGING", "true"),
		helper.EnvVarFromSecret("ANON_KEY", JWTSecretName(project), JWTSecretAnonKey),
	}
}
