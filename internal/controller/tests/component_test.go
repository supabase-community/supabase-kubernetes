package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	authcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/auth"
	componentcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	edgeruntimecontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/edgeruntime"
	envoycontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/envoy"
	metacontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/meta"
	realtimecontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/realtime"
	restcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/rest"
	storagecontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/storage"
	studiocontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/studio"
	supavisorcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/supavisor"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type componentObject interface {
	reconciler.ConditionedObject
	GetProjectRef() corev1.LocalObjectReference
}

const (
	kindEdgeRuntime = "EdgeRuntime"
	kindStorage     = "Storage"
	kindStudio      = "Studio"
)

type componentTestClient struct{ client.Client }

func (r *componentTestClient) reconcile(ctx context.Context, obj componentObject) (ctrl.Result, error) {
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(obj)}
	switch obj.(type) {
	case *core.Auth:
		return (&authcontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Rest:
		return (&restcontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Meta:
		return (&metacontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Supavisor:
		return (&supavisorcontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Realtime:
		return (&realtimecontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Storage:
		return (&storagecontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Studio:
		return (&studiocontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.Envoy:
		return (&envoycontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	case *core.EdgeRuntime:
		return (&edgeruntimecontroller.Reconciler{Client: r.Client}).Reconcile(ctx, req)
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported component %T", obj)
	}
}

func componentFixture(t *testing.T) (context.Context, *componentTestClient, *core.Project, []componentObject) {
	t.Helper()
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{core.AddToScheme, corev1.AddToScheme, appsv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	p := &core.Project{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "test", UID: "project"}, Spec: core.ProjectSpec{JWTExpSec: ptr.To(int32(3600)), PublicURL: "https://api.test.local:8443", DatabaseRef: core.DatabaseRef{Kind: "SingleDatabase", Name: "db"}}}
	reconciler.SetReady(p, "TestReady", "Shared infrastructure ready")
	db := &core.SingleDatabase{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "test"}, Status: core.SingleDatabaseStatus{ResolvedDatabase: &core.ResolvedDatabase{Host: "db", Port: 5432, DBName: "postgres", User: "postgres", PasswordRef: core.SecretKeyRef{Name: "db-password", Key: "password"}}}}
	reconciler.SetReady(db, "TestReady", "Database ready")
	b := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(p, db, &appsv1.Deployment{}, &appsv1.StatefulSet{}).WithObjects(p, db)
	b = b.WithIndex(&core.Analytics{}, componentcontroller.ProjectRefIndex, func(obj client.Object) []string { return []string{obj.(*core.Analytics).Spec.ProjectRef.Name} })
	objects := []componentObject{
		&core.Auth{},
		&core.Rest{},
		&core.Meta{},
		&core.Realtime{},
		&core.Storage{},
		&core.Studio{},
		&core.Envoy{},
		&core.EdgeRuntime{},
		&core.Supavisor{},
	}
	for _, o := range objects {
		kind := strings.TrimPrefix(fmt.Sprintf("%T", o), "*v1alpha1.")
		o.SetName("custom-" + strings.ToLower(kind))
		o.SetNamespace(p.Namespace)
		o.SetUID(types.UID(kind))
		spec := map[string]any{"projectRef": map[string]string{"name": p.Name}}
		if _, fixedReplicas := o.(*core.Supavisor); !fixedReplicas {
			spec["replicas"] = 0
		}
		data, _ := json.Marshal(map[string]any{"spec": spec})
		if err := json.Unmarshal(data, o); err != nil {
			t.Fatal(err)
		}
		b = b.WithStatusSubresource(o).WithObjects(o).WithIndex(o, componentcontroller.ProjectRefIndex, func(obj client.Object) []string { return []string{obj.(componentObject).GetProjectRef().Name} })
	}
	for _, name := range []string{"db-password", "shared-jwt", "shared-keys"} {
		b = b.WithObjects(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.Namespace}, Data: map[string][]byte{"password": []byte("password"), "secret": []byte("jwt")}})
	}
	return ctx, &componentTestClient{Client: b.Build()}, p, objects
}
func runComponent(t *testing.T, ctx context.Context, r *componentTestClient, o componentObject) {
	t.Helper()
	if _, err := r.reconcile(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
		t.Fatal(err)
	}
}
func expectReason(t *testing.T, ctx context.Context, r *componentTestClient, o componentObject, reason string) {
	t.Helper()
	if err := r.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
		t.Fatal(err)
	}
	c := meta.FindStatusCondition(*o.GetConditions(), "Ready")
	if c == nil || c.Reason != reason {
		t.Fatalf("%T condition = %+v, want %s", o, c, reason)
	}
}
func expectEnvValue(t *testing.T, workload client.Object, name, value string) {
	t.Helper()
	var containers []corev1.Container
	switch workload := workload.(type) {
	case *appsv1.Deployment:
		containers = workload.Spec.Template.Spec.Containers
	case *appsv1.StatefulSet:
		containers = workload.Spec.Template.Spec.Containers
	default:
		t.Fatalf("unsupported workload %T", workload)
	}
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == name {
				if env.Value != value {
					t.Fatalf("%T environment variable %s = %q, want %q", workload, name, env.Value, value)
				}
				return
			}
		}
	}
	t.Fatalf("%T environment variable %s is missing", workload, name)
}
func expectEnvAbsent(t *testing.T, workload client.Object, name string) {
	t.Helper()
	var containers []corev1.Container
	switch workload := workload.(type) {
	case *appsv1.Deployment:
		containers = workload.Spec.Template.Spec.Containers
	case *appsv1.StatefulSet:
		containers = workload.Spec.Template.Spec.Containers
	default:
		t.Fatalf("unsupported workload %T", workload)
	}
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == name {
				t.Fatalf("%T environment variable %s should be absent", workload, name)
			}
		}
	}
}

func TestPostgRESTSchemaConfiguration(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	rest := objects[1].(*core.Rest)
	studio := objects[5].(*core.Studio)
	runComponent(t, ctx, r, objects[6]) // Create Envoy credentials for Studio.
	targets := []componentObject{rest, studio}
	workloads := []client.Object{&appsv1.Deployment{}, &appsv1.StatefulSet{}}
	keys := []client.ObjectKey{
		{Namespace: p.Namespace, Name: defaults.ComponentName(rest.Name, "rest")},
		{Namespace: p.Namespace, Name: defaults.ComponentName(studio.Name, "studio")},
	}
	envVars := func(schemas, maxRows, searchPath string) []corev1.EnvVar {
		return []corev1.EnvVar{
			{Name: "PGRST_DB_SCHEMAS", Value: schemas},
			{Name: "PGRST_DB_MAX_ROWS", Value: maxRows},
			{Name: "PGRST_DB_EXTRA_SEARCH_PATH", Value: searchPath},
		}
	}
	check := func(wants ...[]corev1.EnvVar) {
		t.Helper()
		for i, target := range targets {
			runComponent(t, ctx, r, target)
			workload := workloads[i]
			if err := r.Get(ctx, keys[i], workload); err != nil {
				t.Fatal(err)
			}
			var env []corev1.EnvVar
			switch w := workload.(type) {
			case *appsv1.Deployment:
				env = w.Spec.Template.Spec.Containers[0].Env
			case *appsv1.StatefulSet:
				env = w.Spec.Template.Spec.Containers[0].Env
			}
			for _, want := range wants[i] {
				var matches []corev1.EnvVar
				for _, got := range env {
					if got.Name == want.Name {
						matches = append(matches, got)
					}
				}
				if len(matches) != 1 || !reflect.DeepEqual(matches[0], want) {
					t.Fatalf("%T: got %+v, want %+v", target, matches, want)
				}
			}
			rv := workload.GetResourceVersion()
			runComponent(t, ctx, r, target)
			if err := r.Get(ctx, keys[i], workload); err != nil {
				t.Fatal(err)
			}
			if rv != workload.GetResourceVersion() {
				t.Fatalf("%T did not stabilize", target)
			}
		}
	}
	updateProject := func(spec *core.ProjectPostgRESTSpec) {
		t.Helper()
		if err := r.Get(ctx, client.ObjectKeyFromObject(p), p); err != nil {
			t.Fatal(err)
		}
		p.Spec.PostgREST = spec
		if err := r.Update(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		spec *core.ProjectPostgRESTSpec
		want []corev1.EnvVar
	}{
		{"absent", nil, envVars("public,graphql_public", "1000", "public")},
		{"empty", &core.ProjectPostgRESTSpec{}, envVars("public,graphql_public", "1000", "public")},
		{"partial", &core.ProjectPostgRESTSpec{MaxRows: ptr.To(int64(2000))}, envVars("public,graphql_public", "2000", "public")},
		{"custom", &core.ProjectPostgRESTSpec{Schemas: ptr.To("public,custom"), MaxRows: ptr.To(int64(2500)), ExtraSearchPath: ptr.To("public,extensions")}, envVars("public,custom", "2500", "public,extensions")},
		{"empty search path", &core.ProjectPostgRESTSpec{ExtraSearchPath: ptr.To("")}, envVars("public,graphql_public", "1000", "")},
		{"removed", nil, envVars("public,graphql_public", "1000", "public")},
	} {
		t.Log(tc.name)
		updateProject(tc.spec)
		check(tc.want, tc.want)
	}
	shared := &core.ProjectPostgRESTSpec{Schemas: ptr.To("public,shared"), MaxRows: ptr.To(int64(3000)), ExtraSearchPath: ptr.To("extensions")}
	updateProject(shared)
	sharedEnv := envVars("public,shared", "3000", "extensions")
	if err := r.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "local-config", Namespace: p.Namespace}, Data: map[string]string{"schemas": "public,local"}}); err != nil {
		t.Fatal(err)
	}
	local := envVars("public,local", "4000", "")
	local[0].Value = ""
	local[0].ValueFrom = &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "local-config"}, Key: "schemas"}}
	for i, target := range targets {
		switch target := target.(type) {
		case *core.Rest:
			target.Spec.Config = local
		case *core.Studio:
			target.Spec.Config = local
		}
		if err := r.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		wants := [][]corev1.EnvVar{sharedEnv, sharedEnv}
		wants[i] = local
		check(wants...)
		// Pod overlays retain final precedence over Project and component config.
		containerName := "rest"
		if i == 1 {
			containerName = "studio"
		}
		pod := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: containerName, Env: []corev1.EnvVar{{Name: "PGRST_DB_MAX_ROWS", Value: "5000"}}}}}}
		switch target := target.(type) {
		case *core.Rest:
			target.Spec.Pod = pod
		case *core.Studio:
			target.Spec.Pod = pod
		}
		if err := r.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		overlayEnv := append([]corev1.EnvVar(nil), local...)
		overlayEnv[1].Value = "5000"
		wants[i] = overlayEnv
		check(wants...)
		switch target := target.(type) {
		case *core.Rest:
			target.Spec.Config = nil
			target.Spec.Pod = nil
		case *core.Studio:
			target.Spec.Config = nil
			target.Spec.Pod = nil
		}
		if err := r.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		check(sharedEnv, sharedEnv)
	}
}

func TestComponentLifecycle(t *testing.T) {
	for _, kind := range []string{"Auth", "Rest", "Meta", "Realtime", "Supavisor", kindStorage, kindStudio, "Envoy", kindEdgeRuntime} {
		t.Run(kind, func(t *testing.T) {
			ctx, r, p, objects := componentFixture(t)
			// Envoy credentials must exist before Studio consumes them.
			for _, o := range objects {
				if _, ok := o.(*core.Envoy); ok {
					runComponent(t, ctx, r, o)
				}
			}
			var target componentObject
			for _, o := range objects {
				if strings.TrimPrefix(fmt.Sprintf("%T", o), "*v1alpha1.") == kind {
					target = o
				}
			}
			runComponent(t, ctx, r, target)
			if pooler, ok := target.(*core.Supavisor); ok {
				expectReason(t, ctx, r, pooler, "WorkloadNotReady")
				markSupavisorReady(t, ctx, r, pooler)
				runComponent(t, ctx, r, pooler)
			}
			expectReason(t, ctx, r, target, "ReconcileSucceeded")
			var workload client.Object = &appsv1.Deployment{}
			suffix := strings.ToLower(kind)
			if kind == kindEdgeRuntime {
				suffix = "edge-runtime"
			}
			if kind == kindStorage || kind == kindStudio {
				workload = &appsv1.StatefulSet{}
			}
			key := client.ObjectKey{Namespace: p.Namespace, Name: defaults.ComponentName(target.GetName(), suffix)}
			if err := r.Get(ctx, key, workload); err != nil {
				t.Fatal(err)
			}
			if !metav1.IsControlledBy(workload, target) {
				t.Fatal("workload owner is not component")
			}
			switch kind {
			case "Auth":
				expectEnvValue(t, workload, "API_EXTERNAL_URL", p.Spec.PublicURL+"/auth/v1")
				expectEnvValue(t, workload, "GOTRUE_JWT_ISSUER", p.Spec.PublicURL+"/auth/v1")
			case kindStorage:
				expectEnvValue(t, workload, "STORAGE_PUBLIC_URL", p.Spec.PublicURL)
			case kindStudio, kindEdgeRuntime:
				expectEnvValue(t, workload, "SUPABASE_PUBLIC_URL", p.Spec.PublicURL)
			}
			rv := workload.GetResourceVersion()
			runComponent(t, ctx, r, target)
			if err := r.Get(ctx, key, workload); err != nil {
				t.Fatal(err)
			}
			if rv != workload.GetResourceVersion() {
				t.Fatal("idempotent reconciliation changed workload")
			}
			rv = updateComponentConfig(t, ctx, r, target, key, workload)
			duplicate := target.DeepCopyObject().(componentObject)
			duplicate.SetName("duplicate")
			duplicate.SetResourceVersion("")
			duplicate.SetUID("duplicate")
			if err := r.Create(ctx, duplicate); err != nil {
				t.Fatal(err)
			}
			runComponent(t, ctx, r, target)
			runComponent(t, ctx, r, duplicate)
			expectReason(t, ctx, r, target, "ConflictingComponents")
			expectReason(t, ctx, r, duplicate, "ConflictingComponents")
			if err := r.Get(ctx, key, workload); err != nil {
				t.Fatal(err)
			}
			if rv != workload.GetResourceVersion() {
				t.Fatal("conflict modified workload")
			}
			if err := r.Delete(ctx, duplicate); err != nil {
				t.Fatal(err)
			}
			runComponent(t, ctx, r, target)
			expectReason(t, ctx, r, target, "ReconcileSucceeded")
			if err := r.Delete(ctx, p); err != nil {
				t.Fatal(err)
			}
			runComponent(t, ctx, r, target)
			expectReason(t, ctx, r, target, "ProjectNotFound")
			if err := r.Get(ctx, key, workload); err != nil {
				t.Fatal(err)
			}
			if rv != workload.GetResourceVersion() {
				t.Fatal("Project deletion modified workload")
			}
		})
	}
}
func TestDependencyRecoveryAndIsolation(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	storage := objects[4]
	rest := objects[1]
	if err := r.Delete(ctx, rest); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, storage)
	expectReason(t, ctx, r, storage, "DependencyNotFound")
	rest.SetResourceVersion("")
	if err := r.Create(ctx, rest); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, storage)
	expectReason(t, ctx, r, storage, "ReconcileSucceeded")
	other := rest.DeepCopyObject().(*core.Rest)
	other.Name = "other"
	other.ResourceVersion = ""
	other.UID = "other"
	other.Spec.ProjectRef.Name = "different-project"
	if err := r.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, rest)
	expectReason(t, ctx, r, rest, "ReconcileSucceeded")
	other.Namespace = "other-namespace"
	other.ResourceVersion = ""
	other.Spec.ProjectRef.Name = p.Name
	if err := r.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, rest)
	expectReason(t, ctx, r, rest, "ReconcileSucceeded")
}
func TestConsumedSecretRollout(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	auth := objects[0]
	runComponent(t, ctx, r, auth)
	key := client.ObjectKey{Namespace: p.Namespace, Name: "custom-auth-auth"}
	d := &appsv1.Deployment{}
	if err := r.Get(ctx, key, d); err != nil {
		t.Fatal(err)
	}
	hash := d.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "shared-jwt"}, secret); err != nil {
		t.Fatal(err)
	}
	secret.Data["secret"] = []byte("rotated")
	if err := r.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, auth)
	if err := r.Get(ctx, key, d); err != nil {
		t.Fatal(err)
	}
	if d.Spec.Template.Annotations["core.supabase.io/inputs-hash"] == hash {
		t.Fatal("Secret rotation did not roll workload")
	}
}

func TestRoutesAndRuntimeConfig(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	envoy, edge, studio := objects[6], objects[7], objects[5]
	runComponent(t, ctx, r, envoy)
	configKey := client.ObjectKey{Namespace: p.Namespace, Name: "custom-envoy-envoy-config"}
	config := &corev1.ConfigMap{}
	if err := r.Get(ctx, configKey, config); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(config.Data["cds.yaml"], "custom-rest-rest") || !strings.Contains(config.Data["lds.template.yaml"], "/functions/v1") {
		t.Fatal("discovered routes missing")
	}
	if err := r.Delete(ctx, objects[1]); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, envoy)
	if err := r.Get(ctx, configKey, config); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(config.Data["cds.yaml"], "custom-rest-rest") {
		t.Fatal("deleted component remains routed")
	}
	runComponent(t, ctx, r, edge)
	runComponent(t, ctx, r, studio)
	configKey.Name = "custom-edgeruntime-config"
	if err := r.Get(ctx, configKey, config); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(config, edge) {
		t.Fatal("runtime configuration has wrong owner")
	}
	if config.Data["index.ts"] != assets.MainFunction || config.Data["deno.jsonc"] != assets.FunctionsDenoConfig {
		t.Fatal("runtime configuration differs from embedded defaults")
	}
	rv := config.ResourceVersion
	runComponent(t, ctx, r, edge)
	if err := r.Get(ctx, configKey, config); err != nil {
		t.Fatal(err)
	}
	if config.ResourceVersion != rv {
		t.Fatal("unchanged configuration was rewritten")
	}
	// Drift is repaired; there is no second controller or Function CR involved.
	config.Data["index.ts"] = "invalid router"
	if err := r.Update(ctx, config); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, edge)
	if err := r.Get(ctx, configKey, config); err != nil {
		t.Fatal(err)
	}
	if config.Data["index.ts"] != assets.MainFunction {
		t.Fatal("configuration drift was not repaired")
	}
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "custom-studio-studio"}, sts); err != nil {
		t.Fatal(err)
	}
	for _, volume := range sts.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil {
			t.Fatal("Studio must not mount runtime configuration")
		}
	}
	if err := r.Delete(ctx, config); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, edge)
	if err := r.Get(ctx, configKey, config); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, p); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, edge)
	expectReason(t, ctx, r, edge, "ProjectNotFound")
}

func TestEdgeRuntimePodConfiguration(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(fmt.Sprintf("persistent-cache=%t", persistent), func(t *testing.T) {
			ctx, r, p, objects := componentFixture(t)
			edge := objects[7].(*core.EdgeRuntime)
			// A user function volume must coexist with the operator's file mounts.
			edge.Spec.Pod = &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{Name: "functions", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
				Containers: []corev1.Container{{Name: "edge-runtime", VolumeMounts: []corev1.VolumeMount{
					{Name: "functions", MountPath: "/home/deno/functions"},
				}}},
			}}
			if persistent {
				edge.Spec.Pod.Spec.Volumes = append(edge.Spec.Pod.Spec.Volumes, corev1.Volume{
					Name: "deno-cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "existing-cache"}},
				})
				edge.Spec.Config = []corev1.EnvVar{{Name: "VERIFY_JWT", Value: "true"}}
			}
			if err := r.Update(ctx, edge); err != nil {
				t.Fatal(err)
			}
			runComponent(t, ctx, r, edge)
			deployment := &appsv1.Deployment{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "custom-edgeruntime-edge-runtime"}, deployment); err != nil {
				t.Fatal(err)
			}
			if persistent {
				expectEnvValue(t, deployment, "VERIFY_JWT", "true")
			} else {
				expectEnvAbsent(t, deployment, "VERIFY_JWT")
			}
			container := deployment.Spec.Template.Spec.Containers[0]
			if strings.Join(container.Args, " ") != "start --user-worker-request-idle-timeout 150000 --main-service /home/deno/functions/main" {
				t.Fatalf("unexpected runtime arguments: %v", container.Args)
			}
			mounts := map[string]corev1.VolumeMount{}
			for _, mount := range container.VolumeMounts {
				mounts[mount.MountPath] = mount
			}
			for _, file := range []struct{ path, subPath string }{{"main/index.ts", "index.ts"}, {"deno.jsonc", "deno.jsonc"}} {
				mount := mounts["/home/deno/functions/"+file.path]
				if mount.Name != "edge-runtime-config" || mount.SubPath != file.subPath || !mount.ReadOnly {
					t.Fatalf("runtime file mount is missing or invalid: %+v", mount)
				}
			}
			if mounts["/home/deno/functions"].Name != "functions" || mounts["/root/.cache/deno"].Name != "deno-cache" {
				t.Fatalf("function or cache mount missing: %v", mounts)
			}
			cacheCount := 0
			for _, volume := range deployment.Spec.Template.Spec.Volumes {
				if volume.Name != "deno-cache" {
					continue
				}
				cacheCount++
				if persistent {
					if volume.EmptyDir != nil || volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ClaimName != "existing-cache" {
						t.Fatalf("PVC must replace the default cache source: %+v", volume)
					}
				} else if volume.EmptyDir == nil || volume.PersistentVolumeClaim != nil {
					t.Fatalf("expected emptyDir cache: %+v", volume)
				}
			}
			if cacheCount != 1 {
				t.Fatalf("expected one cache volume, got %d", cacheCount)
			}
		})
	}
}

func TestPVCOwnershipAndForeignOwnership(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	storage := objects[4].(*core.Storage)
	runComponent(t, ctx, r, storage)
	key := client.ObjectKey{Namespace: p.Namespace, Name: "custom-storage-storage-data"}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, key, pvc); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(pvc, storage) {
		t.Fatal("PVC missing component owner")
	}
	storage.Spec.Storage = &corev1.PersistentVolumeClaimSpec{StorageClassName: ptr.To("another-class")}
	if err := r.Update(ctx, storage); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, storage)
	expectReason(t, ctx, r, storage, "ReconcileFailed")
	if err := r.Get(ctx, key, pvc); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(pvc, storage) {
		t.Fatal("immutable update removed PVC owner")
	}
	storage.Spec.Storage = nil
	if err := r.Update(ctx, storage); err != nil {
		t.Fatal(err)
	}
	// A PVC from a previous CR must not be reused implicitly.
	pvc.Annotations["core.supabase.io/owner-uid"] = "previous-owner"
	if err := r.Update(ctx, pvc); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, storage)
	expectReason(t, ctx, r, storage, "ReconcileFailed")
	// Existing Services belonging to another Project/component are never overwritten.
	auth := objects[0]
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "custom-auth-auth", Namespace: p.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: core.GroupVersion.String(), Kind: "Project", Name: p.Name, UID: p.UID, Controller: ptr.To(true)}}}}
	if err := r.Create(ctx, svc); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, auth)
	expectReason(t, ctx, r, auth, "ReconcileFailed")
	if err := r.Get(ctx, client.ObjectKeyFromObject(svc), svc); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(svc, p) {
		t.Fatal("foreign resource was adopted")
	}
}

func updateComponentConfig(t *testing.T, ctx context.Context, r *componentTestClient, target componentObject, key client.ObjectKey, workload client.Object) string {
	t.Helper()
	rv := workload.GetResourceVersion()
	// Every controller must propagate a spec update into its own Pod template.
	config := []corev1.EnvVar{{Name: "CONTRACT_SETTING", Value: "updated"}}
	switch target.(type) {
	case *core.Auth:
		config = append(config, corev1.EnvVar{Name: "GOTRUE_SITE_URL", Value: "http://localhost:3000"})
	case *core.EdgeRuntime:
		config = append(config, corev1.EnvVar{Name: "VERIFY_JWT", Value: "false"})
	case *core.Storage:
		config = append(config, corev1.EnvVar{Name: "FILE_SIZE_LIMIT", Value: "52428800"})
	case *core.Supavisor:
		config = append(config,
			corev1.EnvVar{Name: "POOLER_DEFAULT_POOL_SIZE", Value: "20"},
			corev1.EnvVar{Name: "POOLER_MAX_CLIENT_CONN", Value: "100"},
			corev1.EnvVar{Name: "DB_POOL_SIZE", Value: "5"},
		)
	}
	for _, env := range config[1:] {
		expectEnvAbsent(t, workload, env.Name)
	}
	data, err := json.Marshal(map[string]any{"spec": map[string]any{"config": config}})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(ctx, target); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, target)
	if err := r.Get(ctx, key, workload); err != nil {
		t.Fatal(err)
	}
	if rv == workload.GetResourceVersion() {
		t.Fatal("component config update did not change workload")
	}
	for _, env := range config {
		expectEnvValue(t, workload, env.Name, env.Value)
	}
	rv = workload.GetResourceVersion()
	runComponent(t, ctx, r, target)
	if err := r.Get(ctx, key, workload); err != nil {
		t.Fatal(err)
	}
	if rv != workload.GetResourceVersion() {
		t.Fatal("updated workload did not stabilize")
	}
	if len(config) > 1 {
		if err := json.Unmarshal([]byte(`{"spec":{"config":[{"name":"CONTRACT_SETTING","value":"updated"}]}}`), target); err != nil {
			t.Fatal(err)
		}
		if err := r.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
		runComponent(t, ctx, r, target)
		if err := r.Get(ctx, key, workload); err != nil {
			t.Fatal(err)
		}
		for _, env := range config[1:] {
			expectEnvAbsent(t, workload, env.Name)
		}
		rv = workload.GetResourceVersion()
	}
	return rv
}
