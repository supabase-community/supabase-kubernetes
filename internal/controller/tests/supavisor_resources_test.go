package tests

import (
	"context"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	supavisordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/supavisor"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSupavisorCredentialsAndRollouts(t *testing.T) {
	g := NewWithT(t)
	ctx, r, project, _ := componentFixture(t)
	pooler := &core.Supavisor{}
	g.Expect(r.Get(ctx, client.ObjectKey{Namespace: project.Namespace, Name: "custom-supavisor"}, pooler)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	state := defaults.NewContext(project)
	state.Names["Supavisor"] = pooler.Name
	deployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: pooler.Namespace, Name: supavisordefaults.SupavisorDeploymentName(state)}
	g.Expect(r.Get(ctx, key, deployment)).To(Succeed())
	g.Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))
	g.Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(1))))
	expectEnvValue(t, deployment, "POOLER_TENANT_ID", project.Name)
	expectEnvValue(t, deployment, "POOLER_DEFAULT_POOL_SIZE", "20")
	expectEnvValue(t, deployment, "POOLER_MAX_CLIENT_CONN", "100")
	expectEnvValue(t, deployment, "DB_POOL_SIZE", "5")
	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "DATABASE_URL" || env.Name == "POSTGRES_PASSWORD" || env.Name == "VAULT_ENC_KEY" {
			g.Expect(env.Value).To(BeEmpty())
			g.Expect(env.ValueFrom.SecretKeyRef).NotTo(BeNil())
		}
	}
	service := &corev1.Service{}
	g.Expect(r.Get(ctx, client.ObjectKey{Namespace: pooler.Namespace, Name: supavisordefaults.SupavisorServiceName(state)}, service)).To(Succeed())
	g.Expect(service.Spec.Ports).To(HaveLen(2))
	g.Expect(service.Spec.Ports[0].Port).To(Equal(int32(5432)))
	g.Expect(service.Spec.Ports[1].Port).To(Equal(int32(6543)))
	g.Expect(metav1.IsControlledBy(service, pooler)).To(BeTrue())

	previousHash := deployment.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	password := &corev1.Secret{}
	g.Expect(r.Get(ctx, client.ObjectKey{Namespace: pooler.Namespace, Name: "db-password"}, password)).To(Succeed())
	rotated := "space @:/?#%&'\"+password"
	password.Data["password"] = []byte(rotated)
	g.Expect(r.Update(ctx, password)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	derived := &corev1.Secret{}
	g.Expect(r.Get(ctx, client.ObjectKey{Namespace: pooler.Namespace, Name: supavisordefaults.SupavisorSecretName(state)}, derived)).To(Succeed())
	g.Expect(metav1.IsControlledBy(derived, pooler)).To(BeTrue())
	connection, err := url.Parse(string(derived.Data[supavisordefaults.DatabaseURLKey]))
	g.Expect(err).NotTo(HaveOccurred())
	actual, _ := connection.User.Password()
	g.Expect(actual).To(Equal(rotated))
	g.Expect(connection.User.Username()).To(Equal("supabase_admin"))
	g.Expect(connection.Path).To(Equal("/_supabase"))
	g.Expect(r.Get(ctx, key, deployment)).To(Succeed())
	g.Expect(deployment.Spec.Template.Annotations["core.supabase.io/inputs-hash"]).NotTo(Equal(previousHash))
	rv := deployment.ResourceVersion
	secretRV := derived.ResourceVersion
	runComponent(t, ctx, r, pooler)
	g.Expect(r.Get(ctx, key, deployment)).To(Succeed())
	g.Expect(deployment.ResourceVersion).To(Equal(rv))
	g.Expect(r.Get(ctx, client.ObjectKeyFromObject(derived), derived)).To(Succeed())
	g.Expect(derived.ResourceVersion).To(Equal(secretRV))

	// Reconcile drift in the rollout strategy without allowing concurrent Pods.
	deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	deployment.Spec.Replicas = ptr.To(int32(2))
	g.Expect(r.Update(ctx, deployment)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	g.Expect(r.Get(ctx, key, deployment)).To(Succeed())
	g.Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))
	g.Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(1))))

	// ConfigMap references from config also participate in rollout hashing.
	config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "pool-size", Namespace: pooler.Namespace}, Data: map[string]string{"size": "25"}}
	g.Expect(r.Create(ctx, config)).To(Succeed())
	pooler.Spec.Config = []corev1.EnvVar{{Name: "POOLER_DEFAULT_POOL_SIZE", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: config.Name}, Key: "size"}}}}
	g.Expect(r.Update(ctx, pooler)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	g.Expect(r.Get(ctx, key, deployment)).To(Succeed())
	previousHash = deployment.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	config.Data["size"] = "30"
	g.Expect(r.Update(ctx, config)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	g.Expect(r.Get(ctx, key, deployment)).To(Succeed())
	g.Expect(deployment.Spec.Template.Annotations["core.supabase.io/inputs-hash"]).NotTo(Equal(previousHash))
}

func TestSupavisorDependencyRecovery(t *testing.T) {
	g := NewWithT(t)
	ctx, r, project, _ := componentFixture(t)
	pooler := &core.Supavisor{}
	g.Expect(r.Get(ctx, client.ObjectKey{Namespace: project.Namespace, Name: "custom-supavisor"}, pooler)).To(Succeed())
	reconciler.SetNotReady(project, "MigrationsNotReady", "Waiting for migrations")
	g.Expect(r.Status().Update(ctx, project)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	expectReason(t, ctx, r, pooler, "ProjectNotReady")
	reconciler.SetReady(project, "Ready", "Ready")
	g.Expect(r.Status().Update(ctx, project)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	expectReason(t, ctx, r, pooler, "WorkloadNotReady")
	password := &corev1.Secret{}
	g.Expect(r.Get(ctx, client.ObjectKey{Namespace: pooler.Namespace, Name: "db-password"}, password)).To(Succeed())
	delete(password.Data, "password")
	g.Expect(r.Update(ctx, password)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	expectReason(t, ctx, r, pooler, "ReconcileFailed")
	password.Data["password"] = []byte("recovered")
	g.Expect(r.Update(ctx, password)).To(Succeed())
	runComponent(t, ctx, r, pooler)
	markSupavisorReady(t, ctx, r, pooler)
	runComponent(t, ctx, r, pooler)
	expectReason(t, ctx, r, pooler, "ReconcileSucceeded")
}

// markSupavisorReady simulates the Deployment controller in tests using a fake client.
func markSupavisorReady(t *testing.T, ctx context.Context, c client.Client, pooler *core.Supavisor) {
	t.Helper()
	g := NewWithT(t)
	deployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: pooler.Namespace, Name: defaults.ComponentName(pooler.Name, "supavisor")}
	g.Expect(c.Get(ctx, key, deployment)).To(Succeed())
	deployment.Status.ObservedGeneration = deployment.Generation
	deployment.Status.Replicas = 1
	deployment.Status.ReadyReplicas = 1
	deployment.Status.UpdatedReplicas = 1
	deployment.Status.AvailableReplicas = 1
	g.Expect(c.Status().Update(ctx, deployment)).To(Succeed())
}
