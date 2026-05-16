package controller

import (
	"fmt"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	proxyAPIComponent    = "proxy-api"
	proxyStudioComponent = "proxy-studio"
	proxyPort            = int32(8080)
	proxyConfigFile      = "envoy.yaml"
	proxyConfigMountPath = "/etc/envoy"
)

type proxyRoute struct {
	Prefix        string
	Cluster       string
	RewritePrefix *string
}

func strPtr(v string) *string { return &v }

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

func buildEnvoyConfig(routes []proxyRoute, clusters []envoyCluster) (string, error) {
	envoyRoutes := make([]map[string]any, 0, len(routes))
	for _, r := range routes {
		route := map[string]any{
			"match": map[string]any{"prefix": r.Prefix},
			"route": map[string]any{"cluster": r.Cluster},
		}
		if r.RewritePrefix != nil {
			route["route"].(map[string]any)["prefix_rewrite"] = *r.RewritePrefix
		}
		envoyRoutes = append(envoyRoutes, route)
	}

	envoyClusters := make([]map[string]any, 0, len(clusters))
	for _, c := range clusters {
		cluster := map[string]any{
			"name":            c.Name,
			"connect_timeout": "5s",
			"type":            "STRICT_DNS",
			"lb_policy":       "ROUND_ROBIN",
			"load_assignment": map[string]any{
				"cluster_name": c.Name,
				"endpoints": []map[string]any{
					{
						"lb_endpoints": []map[string]any{
							{
								"endpoint": map[string]any{
									"address": map[string]any{
										"socket_address": map[string]any{
											"address":    c.Address,
											"port_value": c.Port,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		envoyClusters = append(envoyClusters, cluster)
	}

	config := map[string]any{
		"static_resources": map[string]any{
			"listeners": []map[string]any{
				{
					"address": map[string]any{
						"socket_address": map[string]any{
							"address":    "0.0.0.0",
							"port_value": proxyPort,
						},
					},
					"filter_chains": []map[string]any{
						{
							"filters": []map[string]any{
								{
									"name": "envoy.filters.network.http_connection_manager",
									"typed_config": map[string]any{
										"@type":       "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
										"stat_prefix": "ingress_http",
										"route_config": map[string]any{
											"name": "local_route",
											"virtual_hosts": []map[string]any{
												{
													"name":    "backend",
													"domains": []string{"*"},
													"routes":  envoyRoutes,
												},
											},
										},
										"http_filters": []map[string]any{
											{
												"name": "envoy.filters.http.router",
												"typed_config": map[string]any{
													"@type": "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"clusters": envoyClusters,
		},
	}

	bytes, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshalling envoy config: %w", err)
	}
	return string(bytes), nil
}

type envoyCluster struct {
	Name    string
	Address string
	Port    int32
}

func buildAPIProxyRoutesAndClusters(project *platformv1alpha1.Project) ([]proxyRoute, []envoyCluster) {
	ns := project.Namespace
	name := project.Name

	routes := []proxyRoute{}
	clusters := []envoyCluster{}

	authEnabled := project.Spec.Auth == nil || derefBool(project.Spec.Auth.Enabled, true)
	if authEnabled {
		routes = append(routes, proxyRoute{Prefix: "/auth/v1", Cluster: "auth", RewritePrefix: strPtr("/")})
		clusters = append(clusters, envoyCluster{Name: "auth", Address: componentServiceName(name, "auth") + "." + ns + ".svc.cluster.local", Port: 9999})
	}

	restEnabled := project.Spec.Rest == nil || derefBool(project.Spec.Rest.Enabled, true)
	if restEnabled {
		routes = append(routes, proxyRoute{Prefix: "/rest/v1", Cluster: "rest", RewritePrefix: strPtr("/")})
		routes = append(routes, proxyRoute{Prefix: "/graphql/v1", Cluster: "rest", RewritePrefix: strPtr("/rpc/graphql")})
		clusters = append(clusters, envoyCluster{Name: "rest", Address: componentServiceName(name, "rest") + "." + ns + ".svc.cluster.local", Port: 3000})
	}

	realtimeEnabled := project.Spec.Realtime == nil || derefBool(project.Spec.Realtime.Enabled, true)
	if realtimeEnabled {
		routes = append(routes, proxyRoute{Prefix: "/realtime/v1/api", Cluster: "realtime", RewritePrefix: strPtr("/api")})
		routes = append(routes, proxyRoute{Prefix: "/realtime/v1", Cluster: "realtime", RewritePrefix: strPtr("/socket")})
		clusters = append(clusters, envoyCluster{Name: "realtime", Address: componentServiceName(name, "realtime") + "." + ns + ".svc.cluster.local", Port: 4000})
	}

	storageEnabled := project.Spec.Storage == nil || derefBool(project.Spec.Storage.Enabled, true)
	if storageEnabled {
		routes = append(routes, proxyRoute{Prefix: "/storage/v1", Cluster: "storage", RewritePrefix: strPtr("/")})
		clusters = append(clusters, envoyCluster{Name: "storage", Address: componentServiceName(name, "storage") + "." + ns + ".svc.cluster.local", Port: 5000})
	}

	functionsEnabled := project.Spec.Functions == nil || derefBool(project.Spec.Functions.Enabled, true)
	if functionsEnabled {
		routes = append(routes, proxyRoute{Prefix: "/functions/v1", Cluster: "functions", RewritePrefix: strPtr("/")})
		clusters = append(clusters, envoyCluster{Name: "functions", Address: componentServiceName(name, "functions") + "." + ns + ".svc.cluster.local", Port: 9000})
	}

	metaEnabled := project.Spec.Meta == nil || derefBool(project.Spec.Meta.Enabled, true)
	if metaEnabled {
		routes = append(routes, proxyRoute{Prefix: "/pg", Cluster: "meta", RewritePrefix: strPtr("/")})
		clusters = append(clusters, envoyCluster{Name: "meta", Address: componentServiceName(name, "meta") + "." + ns + ".svc.cluster.local", Port: 8080})
	}

	return routes, clusters
}

func buildStudioProxyRoutesAndClusters(project *platformv1alpha1.Project) ([]proxyRoute, []envoyCluster) {
	ns := project.Namespace
	name := project.Name

	routes := []proxyRoute{
		{Prefix: "/", Cluster: "studio"},
	}
	clusters := []envoyCluster{
		{Name: "studio", Address: componentServiceName(name, "studio") + "." + ns + ".svc.cluster.local", Port: 3000},
	}

	return routes, clusters
}

func buildProxyConfigMap(project *platformv1alpha1.Project, proxyType string) (*corev1.ConfigMap, error) {
	var routes []proxyRoute
	var clusters []envoyCluster

	switch proxyType {
	case proxyAPIComponent:
		routes, clusters = buildAPIProxyRoutesAndClusters(project)
	case proxyStudioComponent:
		routes, clusters = buildStudioProxyRoutesAndClusters(project)
	default:
		return nil, fmt.Errorf("unknown proxy type: %s", proxyType)
	}

	config, err := buildEnvoyConfig(routes, clusters)
	if err != nil {
		return nil, err
	}

	labels := proxyLabels(project, proxyType)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyServiceName(project.Name, proxyType),
			Namespace: project.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			proxyConfigFile: config,
		},
	}
	return cm, nil
}

func buildProxyDeployment(project *platformv1alpha1.Project, proxyType string, image string, replicas *int32, resources corev1.ResourceRequirements) *appsv1.Deployment {
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
					Volumes: []corev1.Volume{{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: proxyServiceName(project.Name, proxyType),
								},
							},
						},
					}},
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
