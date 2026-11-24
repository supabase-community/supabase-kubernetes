package tests

import (
	"testing"

	. "github.com/onsi/gomega"
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestStudioFunctionsDirectory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		volume *corev1.Volume
		mount  *corev1.VolumeMount
	}{
		{name: "default empty directory"},
		{
			name: "custom emptyDir and read-only mount",
			volume: &corev1.Volume{Name: "functions", VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory},
			}},
			mount: &corev1.VolumeMount{Name: "functions", MountPath: "/app/edge-functions", ReadOnly: true},
		},
		{
			name: "PVC with default mount",
			volume: &corev1.Volume{Name: "functions", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "user-functions"},
			}},
		},
		{
			name: "PVC with a different volume name",
			volume: &corev1.Volume{Name: "custom-functions", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "user-functions"},
			}},
			mount: &corev1.VolumeMount{Name: "custom-functions", MountPath: "/app/edge-functions", SubPath: "functions"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx, r, p, objects := componentFixture(t)
			studio := objects[5].(*core.Studio)
			runComponent(t, ctx, r, objects[6])               // Create Envoy credentials for Studio.
			g.Expect(r.Delete(ctx, objects[7])).To(Succeed()) // No EdgeRuntime is required.
			wantVolume := corev1.Volume{Name: "functions", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}
			wantMount := corev1.VolumeMount{Name: "functions", MountPath: "/app/edge-functions"}
			if tc.volume != nil {
				wantVolume = *tc.volume
				studio.Spec.Pod = &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{*tc.volume},
					Containers: []corev1.Container{{
						Name: "functions-sync", Image: "example.com/sync:test",
						VolumeMounts: []corev1.VolumeMount{{Name: tc.volume.Name, MountPath: "/functions"}},
					}},
				}}
				if tc.mount != nil {
					wantMount = *tc.mount
					studio.Spec.Pod.Spec.Containers = append(studio.Spec.Pod.Spec.Containers, corev1.Container{
						Name: "studio", VolumeMounts: []corev1.VolumeMount{*tc.mount},
					})
				}
				g.Expect(r.Update(ctx, studio)).To(Succeed())
			}
			runComponent(t, ctx, r, studio)
			key := client.ObjectKey{Namespace: p.Namespace, Name: studio.Name + "-studio"}
			workload := &appsv1.StatefulSet{}
			g.Expect(r.Get(ctx, key, workload)).To(Succeed())
			expectEnvValue(t, workload, "EDGE_FUNCTIONS_MANAGEMENT_FOLDER", "/app/edge-functions")
			pod := workload.Spec.Template.Spec
			var container corev1.Container
			for _, c := range pod.Containers {
				if c.Name == "studio" {
					container = c
				}
			}
			g.Expect(container.VolumeMounts).To(ConsistOf(
				wantMount,
				corev1.VolumeMount{Name: "studio-data", MountPath: "/app/snippets", SubPath: "snippets"},
			))
			g.Expect(pod.Volumes).To(ConsistOf(
				wantVolume,
				corev1.Volume{Name: "studio-data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: studio.Name + "-studio-data",
				}}},
			))
			if tc.volume != nil {
				g.Expect(pod.Containers).To(ContainElement(studio.Spec.Pod.Spec.Containers[0]), "function sync sidecar must be preserved")
			}
			version := workload.ResourceVersion
			runComponent(t, ctx, r, studio)
			g.Expect(r.Get(ctx, key, workload)).To(Succeed())
			g.Expect(workload.ResourceVersion).To(Equal(version), "reconciliation must not rewrite an unchanged StatefulSet")
		})
	}
}
