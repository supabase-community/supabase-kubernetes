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

package studio

import (
	"fmt"
	"strconv"

	"github.com/supabase-community/supabase-kubernetes/internal/defaults"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/function"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// StudioStatefulSetName returns the name of the Studio StatefulSet for a Project.
func StudioStatefulSetName(project *ResourceContext) string {
	return ComponentName(project.Names["Studio"], "studio")
}

// StudioStatefulSet constructs the Studio StatefulSet for a Project.
func StudioStatefulSet(project *ResourceContext, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.StatefulSet, error) {
	if project.Spec.Studio == nil {
		return nil, nil
	}

	base := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: StudioLabels(project)},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{buildStudioContainer(project, db)}, Volumes: []corev1.Volume{buildStudioVolume(project)}},
	}
	function.AddSync(&base.Spec, project.Name, StudioStatefulSetName(project), false)
	template, err := helper.Overlay(base, project.Spec.Studio.Pod)
	if err != nil {
		return nil, err
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
			Template: template,
		},
	}

	return sts, nil
}

// studioReplicas returns the number of Studio replicas from the spec or the default.
func studioReplicas(project *ResourceContext) *int32 {
	if project.Spec.Studio != nil && project.Spec.Studio.Replicas != nil {
		return project.Spec.Studio.Replicas
	}
	return ptr.To(int32(1))
}

// buildStudioContainer returns the Studio container specification.
func buildStudioContainer(project *ResourceContext, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "studio",
		Image:           studioImage(project),
		ImagePullPolicy: studioImagePullPolicy(project),
		Env:             buildStudioEnvVars(project, db),
		Ports:           studioPorts(),
		Resources:       corev1.ResourceRequirements{},
		LivenessProbe:   studioLivenessProbe(),
		ReadinessProbe:  studioReadinessProbe(),
		StartupProbe:    studioStartupProbe(),
		VolumeMounts: []corev1.VolumeMount{
			{Name: "studio-data", MountPath: StudioSnippetsMountPath, SubPath: StudioSnippetsSubPath},
			{Name: function.SyncVolumeName, MountPath: StudioFunctionsMountPath, ReadOnly: true},
		},
	}
}

// studioImage returns the Studio image from the spec or the default image.
func studioImage(project *ResourceContext) string {
	return defaults.DefaultStudioImage
}

// studioImagePullPolicy returns the Studio image pull policy from the spec or the default.
func studioImagePullPolicy(project *ResourceContext) corev1.PullPolicy {
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
func buildStudioEnvVars(project *ResourceContext, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
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
		helper.EnvVar("POSTGRES_USER_READ_WRITE", "postgres"),
		helper.EnvVarFromSecret("PG_META_CRYPTO_KEY", KeysSecretName(project), KeysSecretCryptoKey),
		helper.EnvVar("PGRST_DB_SCHEMAS", restSchemasOrDefault()),
		helper.EnvVar("PGRST_DB_MAX_ROWS", restMaxRowsOrDefault()),
		helper.EnvVar("PGRST_DB_EXTRA_SEARCH_PATH", restExtraSearchPathOrDefault()),
		helper.EnvVar("SUPABASE_URL", fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			EnvoyServiceName(project),
			project.Namespace,
			DefaultEnvoyPort,
		)),
		helper.EnvVar("SUPABASE_PUBLIC_URL", project.Spec.PublicURL),
		helper.EnvVarFromSecret("SUPABASE_ANON_KEY", jwtSecret, JWTSecretAnonKey),
		helper.EnvVarFromSecret("SUPABASE_SERVICE_KEY", jwtSecret, JWTSecretServiceKey),
		helper.EnvVarFromSecret("AUTH_JWT_SECRET", jwtSecret, JWTSecretKey),
		helper.EnvVarFromSecret("SUPABASE_PUBLISHABLE_KEY", jwtSecret, JWTSecretPublishableKey),
		helper.EnvVarFromSecret("SUPABASE_SECRET_KEY", jwtSecret, JWTSecretOpaqueKey),
		helper.EnvVar("ENABLED_FEATURES_LOGS_ALL", "false"),
		helper.EnvVar("SNIPPETS_MANAGEMENT_FOLDER", StudioSnippetsMountPath),
		helper.EnvVar("EDGE_FUNCTIONS_MANAGEMENT_FOLDER", StudioFunctionsMountPath),
	}

	return helper.MergeEnvVars(env, project.Spec.Studio.Config)
}

// buildStudioVolume returns the snippets PVC volume specification.
func buildStudioVolume(project *ResourceContext) corev1.Volume {
	return corev1.Volume{
		Name: "studio-data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: StudioPVCName(project),
			},
		},
	}
}
