package supavisor

import (
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

func SupavisorDeploymentName(ctx *ResourceContext) string {
	return ComponentName(ctx.Names["Supavisor"], "supavisor")
}

func SupavisorDeployment(ctx *ResourceContext, db *core.ResolvedDatabase) (*appsv1.Deployment, error) {
	probe := corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
		Path: "/api/health", Port: intstr.FromString("http"),
	}}
	template, err := helper.Overlay(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: SupavisorLabels(ctx)},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "supavisor", Image: defaults.DefaultSupavisorImage, ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"/bin/sh", "-ec"},
				Args:    []string{`/app/bin/migrate && /app/bin/supavisor eval "$(cat /etc/pooler/pooler.exs)" && exec /app/bin/server`},
				Env:     supavisorEnv(ctx, db),
				Ports: []corev1.ContainerPort{
					{Name: "http", ContainerPort: DefaultSupavisorPort},
					{Name: "session", ContainerPort: DefaultSessionPort},
					{Name: "transaction", ContainerPort: DefaultTransactionPort},
				},
				VolumeMounts:   []corev1.VolumeMount{{Name: "pooler", MountPath: "/etc/pooler", ReadOnly: true}},
				StartupProbe:   &corev1.Probe{ProbeHandler: probe, PeriodSeconds: 10, TimeoutSeconds: 5, FailureThreshold: 30},
				ReadinessProbe: &corev1.Probe{ProbeHandler: probe, PeriodSeconds: 5, TimeoutSeconds: 5, FailureThreshold: 3},
				LivenessProbe:  &corev1.Probe{ProbeHandler: probe, PeriodSeconds: 10, TimeoutSeconds: 5, FailureThreshold: 10},
			}},
			Volumes: []corev1.Volume{{Name: "pooler", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SupavisorConfigMapName(ctx)}},
			}}},
		},
	}, ctx.Spec.Supavisor.Pod)
	if err != nil {
		return nil, err
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: SupavisorDeploymentName(ctx), Namespace: ctx.Namespace, Labels: SupavisorLabels(ctx)},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: SupavisorSelectorLabels(ctx)},
			Template: template,
		},
	}, nil
}

func supavisorEnv(ctx *ResourceContext, db *core.ResolvedDatabase) []corev1.EnvVar {
	return helper.MergeEnvVars([]corev1.EnvVar{
		helper.EnvVar("PORT", strconv.Itoa(int(DefaultSupavisorPort))),
		helper.EnvVar("POSTGRES_HOST", db.Host),
		helper.EnvVar("POSTGRES_PORT", strconv.Itoa(int(db.Port))),
		helper.EnvVar("POSTGRES_DB", db.DBName),
		helper.EnvVarFromSecret("POSTGRES_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVarFromSecret("DATABASE_URL", SupavisorSecretName(ctx), DatabaseURLKey),
		helper.EnvVar("CLUSTER_POSTGRES", "true"),
		helper.EnvVarFromSecret("SECRET_KEY_BASE", KeysSecretName(ctx), KeysSecretSecretKeyBase),
		helper.EnvVarFromSecret("VAULT_ENC_KEY", KeysSecretName(ctx), KeysSecretVaultEncKey),
		helper.EnvVarFromSecret("API_JWT_SECRET", JWTSecretName(ctx), JWTSecretKey),
		helper.EnvVarFromSecret("METRICS_JWT_SECRET", JWTSecretName(ctx), JWTSecretKey),
		helper.EnvVar("REGION", "local"),
		helper.EnvVar("ERL_AFLAGS", "-proto_dist inet_tcp"),
		helper.EnvVar("POOLER_TENANT_ID", ctx.Name),
		helper.EnvVar("POOLER_DEFAULT_POOL_SIZE", "20"),
		helper.EnvVar("POOLER_MAX_CLIENT_CONN", "100"),
		helper.EnvVar("POOLER_POOL_MODE", "transaction"),
		helper.EnvVar("DB_POOL_SIZE", "5"),
	}, ctx.Spec.Supavisor.Config)
}
