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

// StudioStatefulSetName returns the name of the Studio StatefulSet for a Project.
func StudioStatefulSetName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-studio", project.Name)
}

// StudioStatefulSet constructs the Studio StatefulSet for a Project.
func StudioStatefulSet(project *supabasev1alpha1.Project, functions []supabasev1alpha1.Function, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.StatefulSet, error) {
	if project.Spec.Studio == nil || !*project.Spec.Studio.Enable {
		return nil, nil
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StudioStatefulSetName(project),
			Namespace: project.Namespace,
			Labels:    StudioLabels(project),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: StudioServiceName(project),
			Replicas:    studioReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: StudioSelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      studioPodLabels(project),
					Annotations: studioPodAnnotations(project),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Studio.Affinity,
					NodeSelector:                  project.Spec.Studio.NodeSelector,
					Tolerations:                   project.Spec.Studio.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Studio.PriorityClassName, ""),
					SecurityContext:               project.Spec.Studio.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Studio.TerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						buildStudioContainer(project, functions, db),
					},
					Volumes: buildStudioVolumes(project, functions),
				},
			},
		},
	}

	return sts, nil
}

// studioReplicas returns the number of Studio replicas from the spec or the default.
func studioReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Studio != nil && project.Spec.Studio.Replicas != nil {
		return project.Spec.Studio.Replicas
	}
	return ptr.To(int32(1))
}

// studioPodLabels returns the merged pod labels for the Studio component.
func studioPodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(StudioLabels(project))
	maps.Copy(labels, project.Spec.Studio.PodLabels)
	return labels
}

// studioPodAnnotations returns the merged pod annotations for the Studio component.
func studioPodAnnotations(project *supabasev1alpha1.Project) map[string]string {
	return project.Spec.Studio.PodAnnotations
}

// buildStudioContainer returns the Studio container specification.
func buildStudioContainer(project *supabasev1alpha1.Project, functions []supabasev1alpha1.Function, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "studio",
		Image:           studioImage(project),
		ImagePullPolicy: studioImagePullPolicy(project),
		Env:             buildStudioEnvVars(project, db),
		Ports:           studioPorts(),
		Resources:       ptr.Deref(project.Spec.Studio.Resources, corev1.ResourceRequirements{}),
		LivenessProbe:   studioLivenessProbe(),
		ReadinessProbe:  studioReadinessProbe(),
		StartupProbe:    studioStartupProbe(),
		VolumeMounts:    buildStudioVolumeMounts(functions),
	}
}

// studioImage returns the Studio image from the spec or the default image.
func studioImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Studio.Image != nil && *project.Spec.Studio.Image != "" {
		return *project.Spec.Studio.Image
	}
	return DefaultStudioImage
}

// studioImagePullPolicy returns the Studio image pull policy from the spec or the default.
func studioImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Studio.ImagePullPolicy != nil {
		return *project.Spec.Studio.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// studioPorts returns the container ports for the Studio container.
func studioPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "studio",
			ContainerPort: DefaultStudioPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// studioLivenessProbe returns the liveness probe for the Studio container.
func studioLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        studioProbeHandler(),
		InitialDelaySeconds: 20,
		PeriodSeconds:       5,
		TimeoutSeconds:      10,
		FailureThreshold:    3,
	}
}

// studioReadinessProbe returns the readiness probe for the Studio container.
func studioReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        studioProbeHandler(),
		InitialDelaySeconds: 20,
		PeriodSeconds:       5,
		TimeoutSeconds:      10,
		FailureThreshold:    3,
	}
}

// studioStartupProbe returns the startup probe for the Studio container.
func studioStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        studioProbeHandler(),
		InitialDelaySeconds: 20,
		PeriodSeconds:       5,
		TimeoutSeconds:      10,
		FailureThreshold:    6,
	}
}

// studioProbeHandler returns the shared probe handler for Studio health checks.
func studioProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"node",
				"-e",
				"fetch('http://localhost:3000/api/platform/profile').then((r) => {if (r.status !== 200) throw new Error(r.status)})",
			},
		},
	}
}

// buildStudioEnvVars returns the environment variables for the Studio container.
func buildStudioEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	jwtSecret := JWTSecretName(project)

	env := []corev1.EnvVar{
		helper.EnvVar("HOSTNAME", "0.0.0.0"),
		helper.EnvVar("STUDIO_PG_META_URL", fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			MetaServiceName(project),
			project.Namespace,
			DefaultMetaPort,
		)),
		helper.EnvVar("POSTGRES_PORT", strconv.Itoa(int(db.Port))),
		helper.EnvVar("POSTGRES_HOST", db.Host),
		helper.EnvVar("POSTGRES_DB", db.DBName),
		helper.EnvVarFromSecret("POSTGRES_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVarFromSecret("PG_META_CRYPTO_KEY", KeysSecretName(project), KeysSecretCryptoKey),
		helper.EnvVar("PGRST_DB_SCHEMAS", restSchemasOrDefault(project)),
		helper.EnvVar("PGRST_DB_MAX_ROWS", restMaxRowsOrDefault(project)),
		helper.EnvVar("PGRST_DB_EXTRA_SEARCH_PATH", restExtraSearchPathOrDefault(project)),
		helper.EnvVar("DEFAULT_ORGANIZATION_NAME", project.Spec.Studio.OrgName),
		helper.EnvVar("DEFAULT_PROJECT_NAME", project.Spec.Studio.ProjName),
		helper.EnvVar("SUPABASE_URL", fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			EnvoyServiceName(project),
			project.Namespace,
			DefaultEnvoyPort,
		)),
		helper.EnvVar("SUPABASE_PUBLIC_URL", APIExternalURL(project)),
		helper.EnvVarFromSecret("SUPABASE_ANON_KEY", jwtSecret, JWTSecretAnonKey),
		helper.EnvVarFromSecret("SUPABASE_SERVICE_KEY", jwtSecret, JWTSecretServiceKey),
		helper.EnvVarFromSecret("AUTH_JWT_SECRET", jwtSecret, JWTSecretKey),
		helper.EnvVarFromSecret("SUPABASE_PUBLISHABLE_KEY", jwtSecret, JWTSecretPublishableKey),
		helper.EnvVarFromSecret("SUPABASE_SECRET_KEY", jwtSecret, JWTSecretOpaqueKey),
		helper.EnvVar("ENABLED_FEATURES_LOGS_ALL", "false"),
		helper.EnvVar("SNIPPETS_MANAGEMENT_FOLDER", StudioSnippetsMountPath),
		helper.EnvVar("EDGE_FUNCTIONS_MANAGEMENT_FOLDER", StudioFunctionsMountPath),
	}

	if project.Spec.Studio.OpenAIAPIKey != nil {
		env = append(env, helper.EnvVarFromSecret(
			"OPENAI_API_KEY",
			project.Spec.Studio.OpenAIAPIKey.Name,
			project.Spec.Studio.OpenAIAPIKey.Key,
		))
	}

	return env
}

// buildStudioVolumes returns the volumes for the Studio container.
func buildStudioVolumes(project *supabasev1alpha1.Project, functions []supabasev1alpha1.Function) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(functions)+1)
	volumes = append(volumes, buildStudioVolume(project))
	volumes = append(volumes, buildStudioFunctionVolumes(functions)...)
	return volumes
}

// buildStudioVolume returns the snippets PVC volume specification.
func buildStudioVolume(project *supabasev1alpha1.Project) corev1.Volume {
	return corev1.Volume{
		Name: "studio-data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: StudioPVCName(project),
			},
		},
	}
}

// buildStudioFunctionVolumes returns the ConfigMap volumes for Studio functions.
func buildStudioFunctionVolumes(functions []supabasev1alpha1.Function) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(functions))
	for _, f := range functions {
		volumes = append(volumes, corev1.Volume{
			Name: studioFunctionsVolumeName(&f),
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

// buildStudioVolumeMounts returns the volume mounts for the Studio container.
func buildStudioVolumeMounts(functions []supabasev1alpha1.Function) []corev1.VolumeMount {
	totalFiles := 0
	for _, f := range functions {
		totalFiles += len(f.Spec.Source)
	}

	mounts := make([]corev1.VolumeMount, 0, totalFiles+1)
	mounts = append(mounts, corev1.VolumeMount{
		Name:      "studio-data",
		MountPath: StudioSnippetsMountPath,
		SubPath:   StudioSnippetsSubPath,
	})

	for _, f := range functions {
		for filename := range f.Spec.Source {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      studioFunctionsVolumeName(&f),
				MountPath: fmt.Sprintf("%s/%s/%s", StudioFunctionsMountPath, f.Spec.FunctionName, filename),
				SubPath:   filename,
			})
		}
	}

	return mounts
}

// studioFunctionsVolumeName returns a valid volume name for a Function ConfigMap volume.
func studioFunctionsVolumeName(fn *supabasev1alpha1.Function) string {
	return fmt.Sprintf("studio-function-%s", fn.Name)
}
