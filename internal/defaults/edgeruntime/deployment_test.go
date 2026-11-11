package edgeruntime_test

import (
	"context"
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	edge "github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func runtimeContext() *defaults.Context {
	state := defaults.NewContext(&core.Project{ObjectMeta: metav1.ObjectMeta{Name: "project", Namespace: "test"}})
	state.Spec.EdgeRuntime = &core.EdgeRuntimeSpec{}
	state.Names["EdgeRuntime"] = "functions"
	return state
}

func TestMemoryMonitorMountAndResourceOverlay(t *testing.T) {
	state := runtimeContext()
	db := &core.ResolvedDatabase{}
	deployment, err := edge.FunctionsDeployment(state, db)
	if err != nil {
		t.Fatal(err)
	}
	container := deployment.Spec.Template.Spec.Containers[0]
	if len(container.Resources.Limits) != 0 || len(container.Resources.Requests) != 0 {
		t.Fatal("the operator must not impose memory resources on direct CR users")
	}
	state.Spec.EdgeRuntime.Pod = &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Volumes: []corev1.Volume{{Name: "functions", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		Containers: []corev1.Container{{
			Name:  "edge-runtime",
			Image: "example.com/custom-runtime:test",
			Resources: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4Gi")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
			},
			VolumeMounts: []corev1.VolumeMount{{Name: "functions", MountPath: edge.RuntimeConfigPath}},
		}},
	}}
	deployment, err = edge.FunctionsDeployment(state, db)
	if err != nil {
		t.Fatal(err)
	}
	container = deployment.Spec.Template.Spec.Containers[0]
	if container.Resources.Limits.Memory().Cmp(resource.MustParse("4Gi")) != 0 ||
		container.Resources.Requests.Memory().Cmp(resource.MustParse("256Mi")) != 0 ||
		container.Image != "example.com/custom-runtime:test" {
		t.Fatal("resource and image overrides must survive the operator defaults")
	}
	config := edge.RuntimeConfigMap(state)
	found := false
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == edge.RuntimeConfigPath+"/main/memory-pressure.ts" {
			found = true
			if mount.Name != "edge-runtime-config" || mount.SubPath != "memory-pressure.ts" || !mount.ReadOnly || config.Data[mount.SubPath] == "" {
				t.Fatalf("memory monitor must resolve through an individual ConfigMap file: %+v", mount)
			}
		}
	}
	if !found {
		t.Fatal("function directory overlay removed the memory monitor mount")
	}
}

func TestMemoryMonitorChangesRollPods(t *testing.T) {
	state := runtimeContext()
	config := edge.RuntimeConfigMap(state)
	deployment, err := edge.FunctionsDeployment(state, &core.ResolvedDatabase{})
	if err != nil {
		t.Fatal(err)
	}
	// Isolate the generated configuration volumes from unrelated database/JWT inputs.
	pod := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: deployment.Spec.Template.Spec.Volumes}}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(config).Build()
	ctx := context.Background()
	if err := component.StampInputs(ctx, c, state.Namespace, &pod); err != nil {
		t.Fatal(err)
	}
	before := pod.Annotations["core.supabase.io/inputs-hash"]
	if err := component.StampInputs(ctx, c, state.Namespace, &pod); err != nil {
		t.Fatal(err)
	}
	if pod.Annotations["core.supabase.io/inputs-hash"] != before {
		t.Fatal("unchanged configuration must not trigger another rollout")
	}
	config.Data["memory-pressure.ts"] += "\n// Updated monitor\n"
	if err := c.Update(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := component.StampInputs(ctx, c, state.Namespace, &pod); err != nil {
		t.Fatal(err)
	}
	if pod.Annotations["core.supabase.io/inputs-hash"] == before {
		t.Fatal("a changed memory monitor must trigger a rollout for subPath mounts")
	}
}
