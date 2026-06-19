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
	"github.com/supabase-community/supabase-kubernetes/internal/function"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// FunctionsDeploymentName returns the name of the Functions Deployment for a Project.
func FunctionsDeploymentName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-functions", project.Name)
}

// FunctionsDeployment constructs the Functions Deployment for a Project.
func FunctionsDeployment(project *supabasev1alpha1.Project, functions []supabasev1alpha1.Function, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.Deployment, error) {
	if project.Spec.Functions == nil || !*project.Spec.Functions.Enable {
		return nil, nil
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      FunctionsDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    FunctionsLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: functionsReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: FunctionsSelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      functionsPodLabels(project),
					Annotations: functionsPodAnnotations(project),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Functions.Affinity,
					NodeSelector:                  project.Spec.Functions.NodeSelector,
					Tolerations:                   project.Spec.Functions.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Functions.PriorityClassName, ""),
					SecurityContext:               project.Spec.Functions.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Functions.TerminationGracePeriodSeconds,
					Volumes:                       buildFunctionsVolumes(functions),
					Containers: []corev1.Container{
						buildFunctionsContainer(project, functions, db),
					},
				},
			},
		},
	}

	return deploy, nil
}

// functionsReplicas returns the number of Functions replicas from the spec or the default.
func functionsReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Functions != nil && project.Spec.Functions.Replicas != nil {
		return project.Spec.Functions.Replicas
	}
	return ptr.To(int32(1))
}

// functionsPodLabels returns the merged pod labels for the Functions component.
func functionsPodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(FunctionsLabels(project))
	maps.Copy(labels, project.Spec.Functions.PodLabels)
	return labels
}

// functionsPodAnnotations returns the merged pod annotations for the Functions component.
func functionsPodAnnotations(project *supabasev1alpha1.Project) map[string]string {
	return project.Spec.Functions.PodAnnotations
}

// buildFunctionsContainer returns the Functions container specification.
func buildFunctionsContainer(project *supabasev1alpha1.Project, functions []supabasev1alpha1.Function, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "functions",
		Image:           functionsImage(project),
		ImagePullPolicy: functionsImagePullPolicy(project),
		Args:            []string{"start", "--main-service", "/home/deno/functions/main"},
		Env:             buildFunctionsEnvVars(project, db),
		Ports:           functionsPorts(),
		Resources:       ptr.Deref(project.Spec.Functions.Resources, corev1.ResourceRequirements{}),
		LivenessProbe:   functionsLivenessProbe(),
		ReadinessProbe:  functionsReadinessProbe(),
		StartupProbe:    functionsStartupProbe(),
		VolumeMounts:    buildFunctionsVolumeMounts(functions),
	}
}

// functionsImage returns the Functions image from the spec or the default image.
func functionsImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Functions.Image != nil && *project.Spec.Functions.Image != "" {
		return *project.Spec.Functions.Image
	}
	return DefaultFunctionsImage
}

// functionsImagePullPolicy returns the Functions image pull policy from the spec or the default.
func functionsImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Functions.ImagePullPolicy != nil {
		return *project.Spec.Functions.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// functionsPorts returns the container ports for the Functions container.
func functionsPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "functions",
			ContainerPort: DefaultFunctionsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// functionsLivenessProbe returns the liveness probe for the Functions container.
func functionsLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        functionsProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// functionsReadinessProbe returns the readiness probe for the Functions container.
func functionsReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        functionsProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// functionsStartupProbe returns the startup probe for the Functions container.
func functionsStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        functionsProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
	}
}

// functionsProbeHandler returns the shared probe handler for Functions health checks.
func functionsProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"bash",
				"-c",
				"timeout 1 bash -c '</dev/tcp/127.0.0.1/9000'",
			},
		},
	}
}

// buildFunctionsEnvVars returns the environment variables for the Functions container.
func buildFunctionsEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	return []corev1.EnvVar{
		helper.EnvVarFromSecret("JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVar("SUPABASE_URL", fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			EnvoyServiceName(project),
			project.Namespace,
			DefaultEnvoyPort,
		)),
		helper.EnvVar("SUPABASE_PUBLIC_URL", APIExternalURL(project)),
		helper.EnvVarFromSecret("SUPABASE_ANON_KEY", JWTSecretName(project), JWTSecretAnonKey),
		helper.EnvVarFromSecret("SUPABASE_SERVICE_ROLE_KEY", JWTSecretName(project), JWTSecretServiceKey),
		helper.EnvVarFromSecret("SUPABASE_PUBLISHABLE_KEY", JWTSecretName(project), JWTSecretPublishableKey),
		helper.EnvVarFromSecret("SUPABASE_SECRET_KEY", JWTSecretName(project), JWTSecretOpaqueKey),
		helper.EnvVar("SUPABASE_PUBLISHABLE_KEYS", `{"default":"$(SUPABASE_PUBLISHABLE_KEY)"}`),
		helper.EnvVar("SUPABASE_SECRET_KEYS", `{"default":"$(SUPABASE_SECRET_KEY)"}`),
		helper.EnvVarFromSecret("DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("SUPABASE_DB_URL", fmt.Sprintf(
			"postgresql://postgres:$(DB_PASSWORD)@%s:%s/%s",
			db.Host,
			strconv.Itoa(int(db.Port)),
			db.DBName,
		)),
		helper.EnvVar("VERIFY_JWT", strconv.FormatBool(project.Spec.Functions.VerifyJWT)),
	}
}

// buildFunctionsVolumes returns the ConfigMap volumes for the Functions container.
func buildFunctionsVolumes(functions []supabasev1alpha1.Function) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(functions))
	for _, f := range functions {
		volumes = append(volumes, corev1.Volume{
			Name: functionsVolumeName(&f),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: function.FunctionConfigMapName(&f),
					},
				},
			},
		})
	}
	return volumes
}

// buildFunctionsVolumeMounts returns the volume mounts for the Functions container.
func buildFunctionsVolumeMounts(functions []supabasev1alpha1.Function) []corev1.VolumeMount {
	totalFiles := 0
	for _, f := range functions {
		totalFiles += len(f.Spec.Source)
	}

	mounts := make([]corev1.VolumeMount, 0, totalFiles)
	for _, f := range functions {
		for filename := range f.Spec.Source {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      functionsVolumeName(&f),
				MountPath: fmt.Sprintf("/home/deno/functions/%s/%s", f.Spec.FunctionName, filename),
				SubPath:   filename,
			})
		}
	}
	return mounts
}

// functionsVolumeName returns a valid volume name for a Function ConfigMap volume.
func functionsVolumeName(fn *supabasev1alpha1.Function) string {
	return fmt.Sprintf("function-%s", fn.Name)
}
