package analytics

import (
	"fmt"
	"strconv"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func AnalyticsDeploymentName(c *ResourceContext) string { return AnalyticsServiceName(c) }
func URL(c *ResourceContext) string {
	return fmt.Sprintf("http://%s:%d", helper.ServiceFQDN(AnalyticsServiceName(c), c.Namespace), DefaultAnalyticsPort)
}
func AnalyticsDeployment(c *ResourceContext, db *core.ResolvedDatabase) (*appsv1.Deployment, error) {
	env := []corev1.EnvVar{
		helper.EnvVar("LOGFLARE_NODE_HOST", "127.0.0.1"), helper.EnvVar("DB_USERNAME", db.User),
		helper.EnvVar("DB_DATABASE", "_supabase"), helper.EnvVar("DB_HOSTNAME", db.Host), helper.EnvVar("DB_PORT", strconv.Itoa(int(db.Port))),
		helper.EnvVarFromSecret("DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key), helper.EnvVar("DB_SCHEMA", "_analytics"),
		helper.EnvVar("LOGFLARE_SINGLE_TENANT", "true"), helper.EnvVar("LOGFLARE_SUPABASE_MODE", "true"),
		helper.EnvVarFromSecret("LOGFLARE_PUBLIC_ACCESS_TOKEN", AnalyticsAuthSecretName(c), AuthSecretPublicTokenKey),
		helper.EnvVarFromSecret("LOGFLARE_PRIVATE_ACCESS_TOKEN", AnalyticsAuthSecretName(c), AuthSecretPrivateTokenKey),
		helper.EnvVar("LOGFLARE_FEATURE_FLAG_OVERRIDE", "multibackend=true"),
		helper.EnvVarFromSecret("POSTGRES_BACKEND_URL", AnalyticsAuthSecretName(c), AuthSecretURLKey),
		helper.EnvVar("POSTGRES_BACKEND_SCHEMA", "_analytics"),
	}
	probe := func(threshold int32) *corev1.Probe {
		return &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/health", Port: intstr.FromInt32(4000)}}, PeriodSeconds: 5, TimeoutSeconds: 5, FailureThreshold: threshold}
	}
	pod, err := helper.Overlay(corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: AnalyticsLabels(c)}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "analytics", Image: defaults.DefaultAnalyticsImage, ImagePullPolicy: corev1.PullIfNotPresent, Env: helper.MergeEnvVars(env, c.Spec.Analytics.Config), Ports: []corev1.ContainerPort{{Name: "analytics", ContainerPort: 4000}}, StartupProbe: probe(16), ReadinessProbe: probe(3), LivenessProbe: probe(10)}}}}, c.Spec.Analytics.Pod)
	if err != nil {
		return nil, err
	}
	replicas := c.Spec.Analytics.Replicas
	if replicas == nil {
		replicas = ptr.To(int32(1))
	}
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: AnalyticsDeploymentName(c), Namespace: c.Namespace, Labels: AnalyticsLabels(c)}, Spec: appsv1.DeploymentSpec{Replicas: replicas, Selector: &metav1.LabelSelector{MatchLabels: AnalyticsSelectorLabels(c)}, Template: pod}}, nil
}
