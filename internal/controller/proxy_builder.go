package controller

import (
	_ "embed"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	proxyAPIComponent    = "proxy-api"
	proxyStudioComponent = "proxy-studio"
	proxyPort            = int32(80)
	proxyConfigFile      = "envoy.yaml"
	proxyConfigMountPath = "/etc/envoy"
	proxyTemplatePath    = "/templates"
)

//go:embed proxy_templates/api_proxy.yaml
var apiProxyTemplate string

//go:embed proxy_templates/studio_proxy.yaml
var studioProxyTemplate string

func proxyLabels(project *platformv1alpha1.Project, proxyType string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "supabase",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/component":  proxyType,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

func proxyServiceName(projectName, proxyType string) string {
	return fmt.Sprintf("%s-%s", projectName, proxyType)
}

func serviceHost(projectName, namespace, component string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", componentServiceName(projectName, component), namespace)
}

func proxyConfigMapName(projectName, proxyType string) string {
	return proxyServiceName(projectName, proxyType)
}

func buildProxyConfigMap(project *platformv1alpha1.Project, proxyType string) *corev1.ConfigMap {
	labels := proxyLabels(project, proxyType)
	var tmpl string
	if proxyType == proxyAPIComponent {
		tmpl = apiProxyTemplate
	} else {
		tmpl = studioProxyTemplate
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyConfigMapName(project.Name, proxyType),
			Namespace: project.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			proxyConfigFile: tmpl,
		},
	}
}

func buildInitScript(proxyType string) string {
	vars := []string{
		"AUTH_ADDRESS",
		"REST_ADDRESS",
		"REALTIME_ADDRESS",
		"STORAGE_ADDRESS",
		"FUNCTIONS_ADDRESS",
		"META_ADDRESS",
		"ANON_KEY",
		"ANON_KEY_ASYMMETRIC",
		"SERVICE_ROLE_KEY",
		"SERVICE_ROLE_KEY_ASYMMETRIC",
		"SUPABASE_PUBLISHABLE_KEY",
		"SUPABASE_SECRET_KEY",
	}
	if proxyType == proxyStudioComponent {
		vars = append(vars, "STUDIO_ADDRESS", "DASHBOARD_BASIC_AUTH")
	}

	sedArgs := make([]string, 0, len(vars))
	for _, v := range vars {
		sedArgs = append(sedArgs, fmt.Sprintf(`-e "s|\${%s}|${%s}|g"`, v, v))
	}

	return fmt.Sprintf(
		"cp %s/%s %s/%s && sed -i %s %s/%s",
		proxyTemplatePath, proxyConfigFile,
		proxyConfigMountPath, proxyConfigFile,
		strings.Join(sedArgs, " "),
		proxyConfigMountPath, proxyConfigFile,
	)
}

func buildProxyDeployment(project *platformv1alpha1.Project, proxyType, image string, replicas *int32, resources corev1.ResourceRequirements, env []corev1.EnvVar) *appsv1.Deployment {
	labels := proxyLabels(project, proxyType)
	r := derefInt32(replicas, 1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyServiceName(project.Name, proxyType),
			Namespace: project.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Name:    "envoy-init",
						Image:   image,
						Command: []string{"sh", "-c", buildInitScript(proxyType)},
						Env:     env,
						VolumeMounts: []corev1.VolumeMount{
							{Name: "config-template", MountPath: proxyTemplatePath},
							{Name: "config", MountPath: proxyConfigMountPath},
						},
					}},
					Containers: []corev1.Container{{
						Name:  proxyType,
						Image: image,
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: proxyPort,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: resources,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "config",
							MountPath: proxyConfigMountPath,
						}},
						Args: []string{"-c", proxyConfigMountPath + "/" + proxyConfigFile},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "config-template",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: proxyConfigMapName(project.Name, proxyType),
									},
								},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func buildProxyService(project *platformv1alpha1.Project, proxyType string, serviceType corev1.ServiceType) *corev1.Service {
	labels := proxyLabels(project, proxyType)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyServiceName(project.Name, proxyType),
			Namespace: project.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       proxyPort,
				TargetPort: intstr.FromInt32(proxyPort),
			}},
		},
	}
}
