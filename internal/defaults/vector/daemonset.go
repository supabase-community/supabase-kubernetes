package vector

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// VectorDaemonSet constructs the per-node log collector for a Project.
func VectorDaemonSet(project *ResourceContext) (*appsv1.DaemonSet, error) {
	template, err := helper.Overlay(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: VectorLabels(project)},
		Spec: corev1.PodSpec{
			ServiceAccountName: VectorName(project),
			NodeSelector:       map[string]string{"kubernetes.io/os": "linux"},
			Containers:         []corev1.Container{buildVectorContainer(project)},
			Volumes:            vectorVolumes(project),
		},
	}, project.Spec.Vector.Pod)
	if err != nil {
		return nil, err
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VectorName(project),
			Namespace: project.Namespace,
			Labels:    VectorLabels(project),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: VectorLabels(project)},
			Template: template,
		},
	}, nil
}

func buildVectorContainer(project *ResourceContext) corev1.Container {
	return corev1.Container{
		Name:            "vector",
		Image:           defaults.DefaultVectorImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            []string{"--config", "/etc/vector/vector.yml"},
		Env:             buildVectorEnvVars(project),
		Ports:           []corev1.ContainerPort{{Name: "api", ContainerPort: DefaultVectorPort}},
		ReadinessProbe:  vectorProbe(),
		LivenessProbe:   vectorProbe(),
		StartupProbe:    vectorProbe(),
		SecurityContext: vectorSecurityContext(),
		VolumeMounts:    vectorVolumeMounts(),
	}
}

func buildVectorEnvVars(project *ResourceContext) []corev1.EnvVar {
	return helper.MergeEnvVars([]corev1.EnvVar{
		{
			Name: "VECTOR_SELF_NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
			},
		},
		helper.EnvVarFromSecret("LOGFLARE_PUBLIC_ACCESS_TOKEN", analyticsdefaults.AnalyticsAuthSecretName(project), analyticsdefaults.AuthSecretPublicTokenKey),
	}, project.Spec.Vector.Config)
}

func vectorProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: "/health", Port: intstr.FromInt32(DefaultVectorPort)},
		},
		PeriodSeconds:    5,
		TimeoutSeconds:   5,
		FailureThreshold: 3,
	}
}

func vectorSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsUser:                ptr.To(int64(0)),
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}

func vectorVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "config", MountPath: "/etc/vector", ReadOnly: true},
		{Name: "logs", MountPath: "/var/log/pods", ReadOnly: true},
		{Name: "state", MountPath: "/var/lib/vector"},
	}
}

func vectorVolumes(project *ResourceContext) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: VectorName(project)},
				},
			},
		},
		{
			Name: "logs",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/log/pods",
					Type: ptr.To(corev1.HostPathDirectory),
				},
			},
		},
		{
			Name:         "state",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}
}
