package tests

import (
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRealtimeClusteringAndCookieRollout(t *testing.T) {
	ctx, c, p, objects := componentFixture(t)
	var realtime *core.Realtime
	for _, obj := range objects {
		if r, ok := obj.(*core.Realtime); ok {
			realtime = r
		}
	}
	if realtime == nil {
		t.Fatal("fixture has no Realtime")
	}
	key := client.ObjectKey{Name: realtime.Name + "-realtime", Namespace: p.Namespace}
	deployment := &appsv1.Deployment{}
	for _, replicas := range []int32{1, 3} {
		realtime.Spec.Replicas = ptr.To(replicas)
		if err := c.Update(ctx, realtime); err != nil {
			t.Fatal(err)
		}
		runComponent(t, ctx, c, realtime)
		if err := c.Get(ctx, key, deployment); err != nil {
			t.Fatal(err)
		}
		if *deployment.Spec.Replicas != replicas {
			t.Fatal("replica count changed")
		}
		container := deployment.Spec.Template.Spec.Containers[0]
		assertSecretEnv(t, container, "RELEASE_COOKIE", "shared-keys", projectdefaults.KeysSecretRealtimeReleaseCookie)
		expectEnvValue(t, deployment, "CLUSTER_STRATEGIES", "POSTGRES")
		for _, name := range []string{"POD_IP", "GEN_RPC_SOCKET_IP"} {
			found := false
			for _, env := range container.Env {
				if env.Name == name {
					found = env.ValueFrom != nil && env.ValueFrom.FieldRef != nil && env.ValueFrom.FieldRef.FieldPath == "status.podIP"
				}
			}
			if !found {
				t.Fatalf("%s must use the Pod IP", name)
			}
		}
		for _, env := range container.Env {
			if env.Name == "DNS_NODES" {
				t.Fatal("dummy DNS discovery remains")
			}
		}
	}
	stable := deployment.ResourceVersion
	hash := deployment.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	runComponent(t, ctx, c, realtime)
	if err := c.Get(ctx, key, deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.ResourceVersion != stable {
		t.Fatal("unchanged Realtime rolled out")
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: "shared-keys", Namespace: p.Namespace}, secret); err != nil {
		t.Fatal(err)
	}
	secret.Data[projectdefaults.KeysSecretRealtimeReleaseCookie] = []byte("replacement-cookie")
	if err := c.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, c, realtime)
	if err := c.Get(ctx, key, deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.Spec.Template.Annotations["core.supabase.io/inputs-hash"] == hash {
		t.Fatal("cookie change did not trigger rollout")
	}
	realtime.Spec.Config = []corev1.EnvVar{{Name: "RELEASE_COOKIE", Value: "custom-cookie"}, {Name: "CLUSTER_STRATEGIES", Value: ""}, {Name: "GEN_RPC_SOCKET_IP", Value: "127.0.0.1"}}
	if err := c.Update(ctx, realtime); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, c, realtime)
	if err := c.Get(ctx, key, deployment); err != nil {
		t.Fatal(err)
	}
	for _, env := range realtime.Spec.Config {
		expectEnvValue(t, deployment, env.Name, env.Value)
		for _, actual := range deployment.Spec.Template.Spec.Containers[0].Env {
			if actual.Name == env.Name && actual.ValueFrom != nil {
				t.Fatalf("%s override retained its default source", env.Name)
			}
		}
	}
	realtime.Spec.Pod = &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "realtime", Env: []corev1.EnvVar{{Name: "RELEASE_COOKIE", Value: "pod-cookie"}}}}}}
	if err := c.Update(ctx, realtime); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, c, realtime)
	if err := c.Get(ctx, key, deployment); err != nil {
		t.Fatal(err)
	}
	expectEnvValue(t, deployment, "RELEASE_COOKIE", "pod-cookie")
}
