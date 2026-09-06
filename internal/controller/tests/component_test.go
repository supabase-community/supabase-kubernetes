package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	authcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/auth"
	componentcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	edgeruntimecontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/edgeruntime"
	envoycontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/envoy"
	metacontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/meta"
	realtimecontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/realtime"
	restcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/rest"
	storagecontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/storage"
	studiocontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/studio"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	functiondefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/function"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
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
	b := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(p, db, &core.Function{}, &appsv1.Deployment{}, &appsv1.StatefulSet{}).WithObjects(p, db)
	objects := []componentObject{
		&core.Auth{},
		&core.Rest{},
		&core.Meta{},
		&core.Realtime{},
		&core.Storage{},
		&core.Studio{},
		&core.Envoy{},
		&core.EdgeRuntime{},
	}
	for _, o := range objects {
		kind := strings.TrimPrefix(fmt.Sprintf("%T", o), "*v1alpha1.")
		o.SetName("custom-" + strings.ToLower(kind))
		o.SetNamespace(p.Namespace)
		o.SetUID(types.UID(kind))
		spec := map[string]any{"projectRef": map[string]string{"name": p.Name}, "replicas": 0}
		if kind == kindStorage || kind == kindStudio {
			spec["storage"] = core.VolumeClaim{Size: resource.MustParse("1Gi"), AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, DeletionPolicy: ptr.To(core.DeletionPolicyDelete)}
		}
		data, _ := json.Marshal(map[string]any{"spec": spec})
		if err := json.Unmarshal(data, o); err != nil {
			t.Fatal(err)
		}
		b = b.WithStatusSubresource(o).WithObjects(o).WithIndex(o, componentcontroller.ProjectRefIndex, func(obj client.Object) []string { return []string{obj.(componentObject).GetProjectRef().Name} })
	}
	b = b.WithIndex(&core.Function{}, componentcontroller.ProjectRefIndex, func(o client.Object) []string { return []string{o.(*core.Function).Spec.ProjectRef.Name} })
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
func TestComponentLifecycle(t *testing.T) {
	for _, kind := range []string{"Auth", "Rest", "Meta", "Realtime", kindStorage, kindStudio, "Envoy", kindEdgeRuntime} {
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
			if kind == kindEdgeRuntime {
				functions := &core.FunctionList{}
				if err := r.List(ctx, functions); err != nil {
					t.Fatal(err)
				}
				if len(functions.Items) != 1 {
					t.Fatalf("expected main Function, got %d", len(functions.Items))
				}
				f := &functions.Items[0]
				if !metav1.IsControlledBy(f, target) {
					t.Fatal("main Function has wrong owner")
				}
				reconcileFunction(t, ctx, r, f)
				runComponent(t, ctx, r, target)
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
				expectEnvValue(t, workload, "API_EXTERNAL_URL", p.Spec.PublicURL)
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

func TestRoutesAndFunctionContent(t *testing.T) {
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
	fn := &core.Function{}
	mainKey := client.ObjectKey{Namespace: p.Namespace, Name: "custom-edgeruntime-edge-runtime-main"}
	if err := r.Get(ctx, mainKey, fn); err != nil {
		t.Fatal(err)
	}
	reconcileFunction(t, ctx, r, fn)
	user := &core.Function{ObjectMeta: metav1.ObjectMeta{Name: "user-code", Namespace: p.Namespace, UID: "user-code"}, Spec: core.FunctionSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}, FunctionName: "hello", Source: map[string]string{"z.ts": "z", "index.ts": "before", "a.ts": "a"}}}
	if err := r.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	reconcileFunction(t, ctx, r, user)
	runComponent(t, ctx, r, edge)
	runComponent(t, ctx, r, studio)
	edgeKey := client.ObjectKey{Namespace: p.Namespace, Name: "custom-edgeruntime-edge-runtime"}
	studioKey := client.ObjectKey{Namespace: p.Namespace, Name: "custom-studio-studio"}
	d := &appsv1.Deployment{}
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, edgeKey, d); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(ctx, studioKey, sts); err != nil {
		t.Fatal(err)
	}
	edgeHash := d.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	studioHash := sts.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	rv := d.ResourceVersion
	for range 5 {
		runComponent(t, ctx, r, edge)
	}
	if err := r.Get(ctx, edgeKey, d); err != nil {
		t.Fatal(err)
	}
	if rv != d.ResourceVersion {
		t.Fatal("function ordering is not deterministic")
	}
	// Direct ConfigMap edits are inputs even if Function source has not changed.
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "user-code-function"}, cm); err != nil {
		t.Fatal(err)
	}
	cm.Data["index.ts"] = "after"
	if err := r.Update(ctx, cm); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, edge)
	runComponent(t, ctx, r, studio)
	if err := r.Get(ctx, edgeKey, d); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(ctx, studioKey, sts); err != nil {
		t.Fatal(err)
	}
	if edgeHash == d.Spec.Template.Annotations["core.supabase.io/inputs-hash"] || studioHash == sts.Spec.Template.Annotations["core.supabase.io/inputs-hash"] {
		t.Fatal("code change failed to roll consumers")
	}
	collision := user.DeepCopy()
	collision.Name = "collision"
	collision.UID = "collision"
	collision.ResourceVersion = ""
	if err := r.Create(ctx, collision); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, edge)
	expectReason(t, ctx, r, edge, componentcontroller.ConflictingFunctionsReason)
	if err := r.Delete(ctx, p); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, edge)
	expectReason(t, ctx, r, edge, "ProjectNotFound")
}

func TestRetentionAndForeignOwnership(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	storage := objects[4].(*core.Storage)
	runComponent(t, ctx, r, storage)
	key := client.ObjectKey{Namespace: p.Namespace, Name: "custom-storage-storage-data"}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, key, pvc); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(pvc, storage) {
		t.Fatal("Delete PVC missing component owner")
	}
	storage.Spec.Storage.DeletionPolicy = ptr.To(core.DeletionPolicyRetain)
	if err := r.Update(ctx, storage); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, storage)
	if err := r.Get(ctx, key, pvc); err != nil {
		t.Fatal(err)
	}
	if len(pvc.OwnerReferences) != 0 {
		t.Fatal("Retain PVC still has owner reference")
	}
	storage.Spec.Storage.DeletionPolicy = ptr.To(core.DeletionPolicyDelete)
	if err := r.Update(ctx, storage); err != nil {
		t.Fatal(err)
	}
	runComponent(t, ctx, r, storage)
	if err := r.Get(ctx, key, pvc); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(pvc, storage) {
		t.Fatal("Delete policy did not restore owner")
	}
	// A retained PVC from a previous CR must not be reused implicitly.
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
	if rv == workload.GetResourceVersion() {
		t.Fatal("component config update did not change workload")
	}
	rv = workload.GetResourceVersion()
	runComponent(t, ctx, r, target)
	if err := r.Get(ctx, key, workload); err != nil {
		t.Fatal(err)
	}
	if rv != workload.GetResourceVersion() {
		t.Fatal("updated workload did not stabilize")
	}
	return rv
}

func reconcileFunction(t *testing.T, ctx context.Context, r *componentTestClient, functionObj *core.Function) {
	t.Helper()
	configMap, err := functiondefaults.FunctionConfigMap(functionObj)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Create(ctx, configMap); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatal(err)
	}
	reconciler.SetReady(functionObj, "ReconcileSucceeded", "Function source reconciled successfully")
	if err := r.Status().Update(ctx, functionObj); err != nil {
		t.Fatal(err)
	}
}
