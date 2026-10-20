package tests

import (
	"context"
	"net/url"
	"testing"

	analyticscontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/analytics"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analytics "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const otherProjectName = "other-project"

func logsClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := core.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	b := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&core.Analytics{}, &core.Vector{}).WithObjects(objects...)
	for _, obj := range []component.Object{&core.Analytics{}, &core.Vector{}, &core.Auth{}, &core.Rest{}, &core.Realtime{}, &core.Storage{}, &core.EdgeRuntime{}, &core.Envoy{}} {
		b = b.WithIndex(obj, component.ProjectRefIndex, func(o client.Object) []string { return []string{o.(component.Object).GetProjectRef().Name} })
	}
	return b.Build()
}
func logsProject() *core.Project {
	return &core.Project{ObjectMeta: metav1.ObjectMeta{Name: "project", Namespace: "default", UID: "project-uid"}, Spec: core.ProjectSpec{PublicURL: "http://localhost", DatabaseRef: core.DatabaseRef{Kind: "SingleDatabase", Name: "db"}}, Status: core.ProjectStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Test", LastTransitionTime: metav1.Now()}}}}
}
func logsDatabase() *core.SingleDatabase {
	return &core.SingleDatabase{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"}, Status: core.SingleDatabaseStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Test", LastTransitionTime: metav1.Now()}}, ResolvedDatabase: &core.ResolvedDatabase{Host: "db.default.svc.cluster.local", Port: 5432, DBName: "postgres", User: "supabase_admin", PasswordRef: core.SecretKeyRef{Name: "db-password", Key: "password"}}}}
}
func TestAnalyticsReconcileAndDatabaseConflict(t *testing.T) {
	p := logsProject()
	db := logsDatabase()
	a := &core.Analytics{ObjectMeta: metav1.ObjectMeta{Name: "logs", Namespace: p.Namespace, UID: "analytics-uid"}, Spec: core.AnalyticsSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	state := defaults.NewContext(p)
	c := logsClient(t, p, db, a, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "db-password", Namespace: p.Namespace}, Data: map[string][]byte{"password": []byte("password")}})
	r := &analyticscontroller.Reconciler{Client: c, Scheme: c.Scheme()}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(a)}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	d := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: p.Namespace, Name: "logs-analytics"}
	if err := c.Get(context.Background(), key, d); err != nil {
		t.Fatal(err)
	}
	version := d.ResourceVersion
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(context.Background(), key, d); err != nil {
		t.Fatal(err)
	}
	if d.ResourceVersion != version {
		t.Fatal("unchanged inputs caused rollout")
	}
	state.Spec.Analytics = &a.Spec
	state.Names["Analytics"] = a.Name
	if analytics.URL(state) != "http://logs-analytics.default.svc.cluster.local:4000" {
		t.Fatal(analytics.URL(state))
	}
	p2 := p.DeepCopy()
	p2.Name = otherProjectName
	p2.ResourceVersion = ""
	if err := c.Create(context.Background(), p2); err != nil {
		t.Fatal(err)
	}
	a2 := a.DeepCopy()
	a2.Name = "other-analytics"
	a2.ResourceVersion = ""
	a2.Spec.ProjectRef.Name = p2.Name
	if err := c.Create(context.Background(), a2); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(context.Background(), key, d); err == nil {
		t.Fatal("conflicting backend kept running")
	}
}
func TestAnalyticsURLPasswordEscaping(t *testing.T) {
	password := "p@ss:/?#% ü"
	s, err := analytics.AnalyticsAuthSecret(defaults.NewContext(logsProject()), logsDatabase().Status.ResolvedDatabase, password)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(string(s.Data[analytics.AuthSecretURLKey]))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := u.User.Password()
	if got != password || u.Path != "/_supabase" {
		t.Fatalf("incorrect connection URL: path=%s", u.Path)
	}
}

func assertSecretEnv(t *testing.T, container corev1.Container, name, secret, key string) {
	t.Helper()
	for _, env := range container.Env {
		if env.Name == name && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			ref := env.ValueFrom.SecretKeyRef
			if ref.Name == secret && ref.Key == key {
				return
			}
		}
	}
	t.Fatalf("%s must reference %s/%s", name, secret, key)
}

func TestAnalyticsOwnsCredentialsAndUpdatesConnection(t *testing.T) {
	ctx := context.Background()
	p := logsProject()
	db := logsDatabase()
	a := &core.Analytics{ObjectMeta: metav1.ObjectMeta{Name: "custom", Namespace: p.Namespace, UID: "analytics-uid"}, Spec: core.AnalyticsSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	password := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "db-password", Namespace: p.Namespace}, Data: map[string][]byte{"password": []byte("initial-password")}}
	c := logsClient(t, p, db, a, password)
	r := &analyticscontroller.Reconciler{Client: c, Scheme: c.Scheme()}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(a)}
	secret := &corev1.Secret{}
	workload := &appsv1.Deployment{}
	reconcile := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
		if err := c.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "custom-analytics-auth"}, secret); err != nil {
			t.Fatal(err)
		}
		if err := c.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "custom-analytics"}, workload); err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	if !metav1.IsControlledBy(secret, a) {
		t.Fatal("Analytics must own its credentials")
	}
	if len(secret.Data) != 3 {
		t.Fatal("all Analytics credentials must be stored in one Secret")
	}
	public, private := string(secret.Data[analytics.AuthSecretPublicTokenKey]), string(secret.Data[analytics.AuthSecretPrivateTokenKey])
	if len(public) != 64 || len(private) != 64 || public == private {
		t.Fatal("invalid generated tokens")
	}
	container := workload.Spec.Template.Spec.Containers[0]
	assertSecretEnv(t, container, "LOGFLARE_PUBLIC_ACCESS_TOKEN", secret.Name, analytics.AuthSecretPublicTokenKey)
	assertSecretEnv(t, container, "LOGFLARE_PRIVATE_ACCESS_TOKEN", secret.Name, analytics.AuthSecretPrivateTokenKey)
	assertSecretEnv(t, container, "POSTGRES_BACKEND_URL", secret.Name, analytics.AuthSecretURLKey)
	version, workloadVersion := secret.ResourceVersion, workload.ResourceVersion
	reconcile()
	if secret.ResourceVersion != version || workload.ResourceVersion != workloadVersion {
		t.Fatal("unchanged credentials caused updates")
	}
	oldHash := workload.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	if err := c.Get(ctx, client.ObjectKeyFromObject(password), password); err != nil {
		t.Fatal(err)
	}
	password.Data["password"] = []byte("rotated@:/?#%password")
	if err := c.Update(ctx, password); err != nil {
		t.Fatal(err)
	}
	reconcile()
	if string(secret.Data[analytics.AuthSecretPublicTokenKey]) != public || string(secret.Data[analytics.AuthSecretPrivateTokenKey]) != private {
		t.Fatal("database password rotation changed access tokens")
	}
	connection, err := url.Parse(string(secret.Data[analytics.AuthSecretURLKey]))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := connection.User.Password()
	if got != string(password.Data["password"]) || connection.Path != "/_supabase" {
		t.Fatal("backend connection was not updated")
	}
	if workload.Spec.Template.Annotations["core.supabase.io/inputs-hash"] == oldHash {
		t.Fatal("connection change did not trigger rollout")
	}
}
