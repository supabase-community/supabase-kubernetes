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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// RestDeploymentName returns the name of the Rest Deployment for a Project.
func RestDeploymentName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-rest", project.Name)
}

// RestDeployment constructs the Rest Deployment for a Project.
func RestDeployment(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.Deployment, error) {
	if project.Spec.Rest == nil || !*project.Spec.Rest.Enable {
		return nil, nil
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RestDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    RestLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: restReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: RestSelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      restPodLabels(project),
					Annotations: restPodAnnotations(project),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Rest.Affinity,
					NodeSelector:                  project.Spec.Rest.NodeSelector,
					Tolerations:                   project.Spec.Rest.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Rest.PriorityClassName, ""),
					SecurityContext:               project.Spec.Rest.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Rest.TerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						buildRestContainer(project, db),
					},
				},
			},
		},
	}

	return deploy, nil
}

// restReplicas returns the number of Rest replicas from the spec or the default.
func restReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Rest != nil && project.Spec.Rest.Replicas != nil {
		return project.Spec.Rest.Replicas
	}
	return ptr.To(int32(1))
}

// restPodLabels returns the merged pod labels for the Rest component.
func restPodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(RestLabels(project))
	maps.Copy(labels, project.Spec.Rest.PodLabels)
	return labels
}

// restPodAnnotations returns the merged pod annotations for the Rest component.
func restPodAnnotations(project *supabasev1alpha1.Project) map[string]string {
	return project.Spec.Rest.PodAnnotations
}

// buildRestContainer returns the Rest container specification.
func buildRestContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "rest",
		Image:           restImage(project),
		ImagePullPolicy: restImagePullPolicy(project),
		Command:         []string{"postgrest"},
		Env:             buildRestEnvVars(project, db),
		Ports:           restPorts(),
		Resources:       ptr.Deref(project.Spec.Rest.Resources, corev1.ResourceRequirements{}),
		LivenessProbe:   restLivenessProbe(),
		ReadinessProbe:  restReadinessProbe(),
		StartupProbe:    restStartupProbe(),
	}
}

// restImage returns the Rest image from the spec or the default image.
func restImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Rest.Image != nil && *project.Spec.Rest.Image != "" {
		return *project.Spec.Rest.Image
	}
	return DefaultRestImage
}

// restImagePullPolicy returns the Rest image pull policy from the spec or the default.
func restImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Rest.ImagePullPolicy != nil {
		return *project.Spec.Rest.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// restPorts returns the container ports for the Rest container.
func restPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "rest",
			ContainerPort: DefaultRestPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// restLivenessProbe returns the liveness probe for the Rest container.
func restLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        restProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// restReadinessProbe returns the readiness probe for the Rest container.
func restReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        restProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// restStartupProbe returns the startup probe for the Rest container.
func restStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        restProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
	}
}

// restProbeHandler returns the shared probe handler for Rest health checks.
func restProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{"postgrest", "--ready"},
		},
	}
}

// buildRestEnvVars returns the environment variables for the Rest container.
func buildRestEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	return []corev1.EnvVar{
		helper.EnvVarFromSecret("PGRST_DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("PGRST_DB_URI", fmt.Sprintf("postgres://authenticator:$(PGRST_DB_PASSWORD)@%s:%s/%s", db.Host, strconv.Itoa(int(db.Port)), db.DBName)),
		helper.EnvVar("PGRST_DB_SCHEMAS", restSchemasOrDefault(project)),
		helper.EnvVar("PGRST_DB_MAX_ROWS", restMaxRowsOrDefault(project)),
		helper.EnvVar("PGRST_DB_EXTRA_SEARCH_PATH", restExtraSearchPathOrDefault(project)),
		helper.EnvVar("PGRST_DB_ANON_ROLE", "anon"),
		helper.EnvVar("PGRST_ADMIN_SERVER_PORT", strconv.Itoa(int(DefaultRestAdminPort))),
		helper.EnvVar("PGRST_ADMIN_SERVER_HOST", "localhost"),
		helper.EnvVarFromSecret("PGRST_JWT_SECRET", JWTSecretName(project), JWTSecretJWKS),
		helper.EnvVar("PGRST_DB_USE_LEGACY_GUCS", "false"),
		helper.EnvVarFromSecret("PGRST_APP_SETTINGS_JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVar("PGRST_APP_SETTINGS_JWT_EXP", strconv.Itoa(int(*project.Spec.JWTExpSec))),
	}
}

// restSchemasOrDefault returns the Rest DB schemas from the spec or the default.
func restSchemasOrDefault(project *supabasev1alpha1.Project) string {
	if project.Spec.Rest != nil && project.Spec.Rest.DBSchemas != nil {
		return *project.Spec.Rest.DBSchemas
	}
	return "public,storage,graphql_public"
}

// restMaxRowsOrDefault returns the Rest DB max rows from the spec or the default.
func restMaxRowsOrDefault(project *supabasev1alpha1.Project) string {
	if project.Spec.Rest != nil && project.Spec.Rest.DBMaxRows != nil {
		return strconv.Itoa(int(*project.Spec.Rest.DBMaxRows))
	}
	return "1000"
}

// restExtraSearchPathOrDefault returns the Rest DB extra search path from the spec or the default.
func restExtraSearchPathOrDefault(project *supabasev1alpha1.Project) string {
	if project.Spec.Rest != nil && project.Spec.Rest.DBExtraSearchPath != nil {
		return *project.Spec.Rest.DBExtraSearchPath
	}
	return "public"
}
