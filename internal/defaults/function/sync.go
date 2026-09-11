package function

import (
	"strconv"

	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	SyncImage             = "python:3.13-alpine"
	SyncVolumeName        = "functions-data"
	syncCredentialsVolume = "functions-sync-credentials"
)

func SyncServiceAccountName(workloadName string) string {
	return helper.ResourceName(workloadName, "functions-sync")
}

// AddSync adds a stable volume and synchronizer before applying the user's overlay.
func AddSync(pod *corev1.PodSpec, projectName, workloadName string, edge bool) {
	pod.ServiceAccountName = SyncServiceAccountName(workloadName)
	pod.AutomountServiceAccountToken = ptr.To(false)
	pod.SecurityContext = &corev1.PodSecurityContext{FSGroup: ptr.To(int64(1000))}
	pod.Volumes = append(pod.Volumes,
		corev1.Volume{Name: SyncVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		corev1.Volume{Name: syncCredentialsVolume, VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
			DefaultMode: ptr.To(int32(0444)),
			Sources: []corev1.VolumeProjection{
				{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{Path: "token", ExpirationSeconds: ptr.To(int64(3600))}},
				{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "kube-root-ca.crt"}, Items: []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}}}},
			},
		}}},
	)
	pod.InitContainers = append(pod.InitContainers, syncContainer(projectName, true, edge))
	pod.Containers = append(pod.Containers, syncContainer(projectName, false, false))
}

func syncContainer(projectName string, once, requireMain bool) corev1.Container {
	name := "functions-sync"
	if once {
		name += "-init"
	}
	required := ""
	if requireMain {
		required = "main/index.ts"
	}
	return corev1.Container{
		Name: name, Image: SyncImage, ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{"python3", "-u", "-c", assets.FunctionsSyncScript},
		Env: []corev1.EnvVar{
			helper.EnvVar("PROJECT_NAME", projectName),
			{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
			helper.EnvVar("SYNC_INTERVAL_SECONDS", "10"),
			helper.EnvVar("SYNC_REQUEST_TIMEOUT_SECONDS", "10"),
			helper.EnvVar("SYNC_DIR", "/functions"),
			helper.EnvVar("SYNC_ONCE", strconv.FormatBool(once)),
			helper.EnvVar("SYNC_REQUIRED_FILE", required),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: SyncVolumeName, MountPath: "/functions"},
			{Name: syncCredentialsVolume, MountPath: "/var/run/secrets/functions-sync", ReadOnly: true},
		},
		Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("32Mi")}},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: ptr.To(true), RunAsUser: ptr.To(int64(1000)), RunAsGroup: ptr.To(int64(1000)),
			AllowPrivilegeEscalation: ptr.To(false), ReadOnlyRootFilesystem: ptr.To(true),
			Capabilities:   &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
	}
}
