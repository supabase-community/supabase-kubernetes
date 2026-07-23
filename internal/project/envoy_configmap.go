/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package project

import (
	"bytes"
	"fmt"
	"slices"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
)

// envoyCluster holds the data needed to render a single Envoy CDS cluster.
type envoyCluster struct {
	Enabled bool
	Address string
	Port    int32
}

type envoyIPFamilyPolicy struct {
	IPv4 bool
	IPv6 bool
}

// envoyTemplateData holds the data used to render the Envoy CDS/LDS templates.
type envoyTemplateData struct {
	AuthCluster      envoyCluster
	RestCluster      envoyCluster
	RealtimeCluster  envoyCluster
	FunctionsCluster envoyCluster
	StorageCluster   envoyCluster
	MetaCluster      envoyCluster
	StudioCluster    envoyCluster
	RealtimeHost     string
	IPFamilyPolicy   envoyIPFamilyPolicy
}

// EnvoyConfigMapName returns the name of the Envoy ConfigMap for a Project.
func EnvoyConfigMapName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-envoy-config", project.Name)
}

// EnvoyConfigMap constructs the Envoy ConfigMap for a Project.
func EnvoyConfigMap(project *supabasev1alpha1.Project) (*corev1.ConfigMap, error) {
	data, err := renderEnvoyConfigMapData(project)
	if err != nil {
		return nil, fmt.Errorf("rendering envoy config: %w", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EnvoyConfigMapName(project),
			Namespace: project.Namespace,
			Labels:    EnvoyLabels(project),
		},
		Data: data,
	}, nil
}

// renderEnvoyConfigMapData renders the static and templated Envoy configuration files.
func renderEnvoyConfigMapData(project *supabasev1alpha1.Project) (map[string]string, error) {
	data := map[string]string{
		"envoy.yaml": assets.EnvoyBaseTemplate,
	}

	tmplData := buildEnvoyTemplateData(project)

	cds, err := renderEnvoyTemplate("cds", assets.EnvoyCDSTemplate, tmplData)
	if err != nil {
		return nil, fmt.Errorf("rendering cds template: %w", err)
	}
	data["cds.yaml"] = cds

	lds, err := renderEnvoyTemplate("lds", assets.EnvoyLDSTemplate, tmplData)
	if err != nil {
		return nil, fmt.Errorf("rendering lds template: %w", err)
	}
	data["lds.template.yaml"] = lds

	return data, nil
}

// renderEnvoyTemplate renders a single Envoy template with the provided data.
func renderEnvoyTemplate(name, tmpl string, data any) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing %s template: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing %s template: %w", name, err)
	}

	return buf.String(), nil
}

// buildEnvoyTemplateData builds the template data from the Project spec.
func buildEnvoyTemplateData(project *supabasev1alpha1.Project) envoyTemplateData {
	ipFamilyPolicy := envoyIPFamilyPolicy{
		// default
		IPv4: true,
		IPv6: false,
	}
	if *EnvoyServiceIPFamilyPolicy(project) == corev1.IPFamilyPolicyPreferDualStack || *EnvoyServiceIPFamilyPolicy(project) == corev1.IPFamilyPolicyRequireDualStack {
		ipFamilyPolicy.IPv4 = true
		ipFamilyPolicy.IPv6 = true
	} else {
		ipFamilies := EnvoyServiceIPFamilies(project)
		if slices.Contains(ipFamilies, corev1.IPv6Protocol) {
			ipFamilyPolicy.IPv6 = true
		}
		if slices.Contains(ipFamilies, corev1.IPv4Protocol) {
			ipFamilyPolicy.IPv4 = true
		} else if ipFamilyPolicy.IPv6 == true {
			// ipv6 single stack
			ipFamilyPolicy.IPv4 = false
		}
	}
	return envoyTemplateData{
		AuthCluster:      buildEnvoyAuthCluster(project),
		RestCluster:      buildEnvoyRestCluster(project),
		RealtimeCluster:  buildEnvoyRealtimeCluster(project),
		FunctionsCluster: buildEnvoyFunctionsCluster(project),
		MetaCluster:      buildEnvoyMetaCluster(project),
		StorageCluster:   buildEnvoyStorageCluster(project),
		StudioCluster:    buildEnvoyStudioCluster(project),
		RealtimeHost:     envoyRealtimeHost(project),
		IPFamilyPolicy:   ipFamilyPolicy,
	}
}

// buildEnvoyAuthCluster builds the auth cluster template data.
func buildEnvoyAuthCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Auth != nil && *project.Spec.Auth.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, AuthServiceName(project)),
		Port:    DefaultAuthPort,
	}
}

// buildEnvoyRestCluster builds the rest cluster template data.
func buildEnvoyRestCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Rest != nil && *project.Spec.Rest.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, RestServiceName(project)),
		Port:    DefaultRestPort,
	}
}

// buildEnvoyRealtimeCluster builds the realtime cluster template data.
func buildEnvoyRealtimeCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Realtime != nil && *project.Spec.Realtime.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, RealtimeServiceName(project)),
		Port:    DefaultRealtimePort,
	}
}

// buildEnvoyFunctionsCluster builds the functions cluster template data.
func buildEnvoyFunctionsCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Functions != nil && *project.Spec.Functions.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, FunctionsServiceName(project)),
		Port:    DefaultFunctionsPort,
	}
}

// buildEnvoyMetaCluster builds the meta cluster template data.
func buildEnvoyMetaCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Meta != nil && *project.Spec.Meta.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, MetaServiceName(project)),
		Port:    DefaultMetaPort,
	}
}

// buildEnvoyStorageCluster builds the storage cluster template data.
func buildEnvoyStorageCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Storage != nil && *project.Spec.Storage.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, StorageServiceName(project)),
		Port:    DefaultStoragePort,
	}
}

// buildEnvoyStudioCluster builds the studio cluster template data.
func buildEnvoyStudioCluster(project *supabasev1alpha1.Project) envoyCluster {
	enabled := project.Spec.Studio != nil && *project.Spec.Studio.Enable
	return envoyCluster{
		Enabled: enabled,
		Address: envoyServiceHost(project, StudioServiceName(project)),
		Port:    DefaultStudioPort,
	}
}

// envoyServiceHost returns the fully qualified DNS name for a Project service.
func envoyServiceHost(project *supabasev1alpha1.Project, serviceName string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, project.Namespace)
}

// envoyRealtimeHost returns the fully qualified DNS name for the Realtime service.
func envoyRealtimeHost(project *supabasev1alpha1.Project) string {
	return envoyServiceHost(project, RealtimeServiceName(project))
}
