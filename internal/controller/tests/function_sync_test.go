package tests

import (
	"reflect"
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	componentcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	edgedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	functiondefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/function"
	studiodefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/studio"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestFunctionSyncOverlayAndCustomAccount(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	var owner *core.Studio
	for _, obj := range objects {
		if studio, ok := obj.(*core.Studio); ok {
			owner = studio
		}
	}
	overlay := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		ServiceAccountName: "custom-account",
		Containers:         []corev1.Container{{Name: "functions-sync", Image: "custom-python:tag", Env: []corev1.EnvVar{{Name: "SYNC_INTERVAL_SECONDS", Value: "42"}}}},
		InitContainers:     []corev1.Container{{Name: "functions-sync-init", Env: []corev1.EnvVar{{Name: "SYNC_REQUEST_TIMEOUT_SECONDS", Value: "17"}}}},
	}}
	state := defaults.NewContext(p)
	state.Spec.Studio = &core.StudioSpec{Pod: overlay}
	state.Spec.EdgeRuntime = &core.EdgeRuntimeSpec{Pod: overlay}
	state.Names["Studio"] = owner.Name
	state.Names["EdgeRuntime"] = "edge"
	db := &core.ResolvedDatabase{Host: "db", Port: 5432}
	studio, err := studiodefaults.StudioStatefulSet(state, db)
	if err != nil {
		t.Fatal(err)
	}
	edge, err := edgedefaults.FunctionsDeployment(state, db)
	if err != nil {
		t.Fatal(err)
	}
	for _, template := range []corev1.PodTemplateSpec{studio.Spec.Template, edge.Spec.Template} {
		if template.Spec.ServiceAccountName != "custom-account" {
			t.Fatal("account overlay lost")
		}
		for _, container := range template.Spec.Containers {
			if container.Name == "functions-sync" {
				if container.Image != "custom-python:tag" {
					t.Fatal("image overlay lost")
				}
				found := false
				for _, env := range container.Env {
					if env.Name == "SYNC_INTERVAL_SECONDS" && env.Value == "42" {
						found = true
					}
				}
				if !found || len(container.Command) == 0 || len(container.VolumeMounts) != 2 {
					t.Fatal("sync overlay lost defaults or interval")
				}
			}
		}
		found := false
		for _, env := range template.Spec.InitContainers[0].Env {
			if env.Name == "SYNC_REQUEST_TIMEOUT_SECONDS" && env.Value == "17" {
				found = true
			}
		}
		if !found {
			t.Fatal("init overlay lost")
		}
	}
	account := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "custom-account", Namespace: p.Namespace}}
	requireSyncSuccess(t, r.Create(ctx, account))
	requireSyncSuccess(t, componentcontroller.EnsureFunctionSyncAccess(ctx, r.Client, owner, studio.Name, &studio.Spec.Template.Spec))
	requireSyncSuccess(t, r.Get(ctx, client.ObjectKeyFromObject(account), account))
	if len(account.OwnerReferences) != 0 {
		t.Fatal("custom account was adopted")
	}
	binding := &rbacv1.RoleBinding{}
	key := client.ObjectKey{Namespace: p.Namespace, Name: functiondefaults.SyncServiceAccountName(studio.Name)}
	requireSyncSuccess(t, r.Get(ctx, key, binding))
	if len(binding.Subjects) != 1 || binding.Subjects[0].Name != account.Name || !metav1.IsControlledBy(binding, owner) {
		t.Fatal("binding does not reference custom account")
	}
	version := binding.ResourceVersion
	requireSyncSuccess(t, componentcontroller.EnsureFunctionSyncAccess(ctx, r.Client, owner, studio.Name, &studio.Spec.Template.Spec))
	requireSyncSuccess(t, r.Get(ctx, key, binding))
	if binding.ResourceVersion != version {
		t.Fatal("unchanged permissions updated binding")
	}
}

func TestFunctionSyncDoesNotRollWorkloads(t *testing.T) {
	for _, kind := range []string{kindStudio, kindEdgeRuntime} {
		t.Run(kind, func(t *testing.T) {
			ctx, r, p, objects := componentFixture(t)
			var target componentObject
			for _, obj := range objects {
				if _, ok := obj.(*core.Envoy); ok {
					runComponent(t, ctx, r, obj)
				}
				switch obj.(type) {
				case *core.Studio:
					if kind == kindStudio {
						target = obj
					}
				case *core.EdgeRuntime:
					if kind == kindEdgeRuntime {
						target = obj
					}
				}
			}
			runComponent(t, ctx, r, target)
			list := &core.FunctionList{}
			requireSyncSuccess(t, r.List(ctx, list))
			for i := range list.Items {
				reconcileFunction(t, ctx, r, &list.Items[i])
			}
			runComponent(t, ctx, r, target)
			expectReason(t, ctx, r, target, "ReconcileSucceeded")
			suffix := "studio"
			if kind == kindEdgeRuntime {
				suffix = "edge-runtime"
			}
			key := client.ObjectKey{Namespace: p.Namespace, Name: defaults.ComponentName(target.GetName(), suffix)}
			getPod := func() (corev1.PodTemplateSpec, string) {
				t.Helper()
				if kind == kindStudio {
					sts := &appsv1.StatefulSet{}
					requireSyncSuccess(t, r.Get(ctx, key, sts))
					return sts.Spec.Template, sts.ResourceVersion
				}
				deployment := &appsv1.Deployment{}
				requireSyncSuccess(t, r.Get(ctx, key, deployment))
				return deployment.Spec.Template, deployment.ResourceVersion
			}
			before, version := getPod()
			if len(before.Spec.InitContainers) != 1 || len(before.Spec.Containers) != 2 {
				t.Fatal("missing synchronizer containers")
			}
			if before.Spec.AutomountServiceAccountToken == nil || *before.Spec.AutomountServiceAccountToken {
				t.Fatal("application receives API token")
			}
			for _, mount := range before.Spec.Containers[0].VolumeMounts {
				if mount.Name == functiondefaults.SyncVolumeName && (mount.SubPath != "" || !mount.ReadOnly) {
					t.Fatal("function mount must be a read-only directory")
				}
			}
			f := &core.Function{ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: p.Namespace, UID: "hello"}, Spec: core.FunctionSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}, FunctionName: "hello", Source: map[string]string{"index.ts": "first"}}}
			requireSyncSuccess(t, r.Create(ctx, f))
			for _, change := range []string{"create", "edit", "rename", "delete"} {
				requireSyncSuccess(t, r.Get(ctx, client.ObjectKeyFromObject(f), f))
				switch change {
				case "edit":
					f.Spec.Source = map[string]string{"index.ts": "updated", "new.ts": "new"}
				case "rename":
					f.Spec.FunctionName = "renamed"
					f.Spec.Source = map[string]string{"index.ts": "updated"}
				}
				if change == "delete" {
					requireSyncSuccess(t, r.Delete(ctx, f))
				} else {
					requireSyncSuccess(t, r.Update(ctx, f))
					reconcileFunction(t, ctx, r, f)
				}
				runComponent(t, ctx, r, target)
				after, current := getPod()
				if !reflect.DeepEqual(before, after) || current != version {
					t.Fatalf("%s changed pod template or workload", change)
				}
			}
			name := functiondefaults.SyncServiceAccountName(key.Name)
			role := &rbacv1.Role{}
			requireSyncSuccess(t, r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: name}, role))
			if len(role.Rules) != 1 || !reflect.DeepEqual(role.Rules[0].Verbs, []string{"list"}) || !reflect.DeepEqual(role.Rules[0].Resources, []string{"functions"}) || !metav1.IsControlledBy(role, target) {
				t.Fatal("incorrect sync permissions or ownership")
			}
		})
	}
}

func requireSyncSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
