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
	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	// DefaultPostgresImage is the default Postgres image used by Project sync Jobs.
	DefaultPostgresImage = "supabase/postgres:17.6.1.084"

	// DefaultBackoffLimit is the number of retries before marking a sync Job as failed.
	DefaultBackoffLimit int32 = 3

	// DefaultTTLSecondsAfterFinished is the TTL for cleaning up finished sync Jobs.
	DefaultTTLSecondsAfterFinished int32 = 30

	// DefaultMetaImage is the default Meta image.
	DefaultMetaImage = "supabase/postgres-meta:v0.96.6"

	// DefaultMetaPort is the default Meta container port.
	DefaultMetaPort int32 = 8080

	// DefaultRestImage is the default Rest image.
	DefaultRestImage = "postgrest/postgrest:v14.12"

	// DefaultRestPort is the default Rest container port.
	DefaultRestPort int32 = 3000

	// DefaultRestAdminPort is the default Rest admin server port.
	DefaultRestAdminPort int32 = 3001

	// DefaultRealtimeImage is the default Realtime image.
	DefaultRealtimeImage = "supabase/realtime:v2.102.3"

	// DefaultRealtimePort is the default Realtime container port.
	DefaultRealtimePort int32 = 4000

	// DefaultAuthImage is the default Auth image.
	DefaultAuthImage = "supabase/gotrue:v2.189.0"

	// DefaultAuthPort is the default Auth container port.
	DefaultAuthPort int32 = 9999

	// DefaultFunctionsImage is the default Functions image.
	DefaultFunctionsImage = "supabase/edge-runtime:v1.74.0"

	// DefaultFunctionsPort is the default Functions container port.
	DefaultFunctionsPort int32 = 9000

	// DefaultEnvoyImage is the default Envoy image.
	DefaultEnvoyImage = "envoyproxy/envoy:v1.38.0"

	// DefaultEnvoyPort is the default Envoy gateway port.
	DefaultEnvoyPort int32 = 8000

	// DefaultEnvoyAdminPort is the default Envoy admin port.
	DefaultEnvoyAdminPort int32 = 9901

	// DefaultEnvoySecretKeyUsername is the Secret data key that holds the dashboard username.
	DefaultEnvoySecretKeyUsername = "username"

	// DefaultEnvoySecretKeyPassword is the Secret data key that holds the dashboard password.
	DefaultEnvoySecretKeyPassword = "password"

	// EnvoyConfigMountPath is the path where Envoy configuration is mounted.
	EnvoyConfigMountPath = "/etc/envoy"

	// EnvoyConfigSourcePath is the path where the Envoy ConfigMap is mounted.
	EnvoyConfigSourcePath = "/etc/envoy-config"

	// DefaultStudioImage is the default Studio image.
	DefaultStudioImage = "supabase/studio:2026.06.03-sha-0bca601"

	// DefaultStudioPort is the default Studio container port.
	DefaultStudioPort int32 = 3000

	// StudioSnippetsMountPath is the path where Studio snippets are mounted.
	StudioSnippetsMountPath = "/app/snippets"

	// StudioSnippetsSubPath is the subPath used for snippets inside the Studio PVC.
	StudioSnippetsSubPath = "snippets"

	// StudioFunctionsMountPath is the path where Studio edge functions are mounted.
	StudioFunctionsMountPath = "/app/edge-functions"
)

// ProjectLabels returns the common labels for a Project and its resources.
func ProjectLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "project",
		"app.kubernetes.io/component":  "project",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// ProjectSelectorLabels returns the selector labels for the Project.
func ProjectSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "project",
		"app.kubernetes.io/component": "project",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// MetaLabels returns the common labels for the Meta component.
func MetaLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "meta",
		"app.kubernetes.io/component":  "meta",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// MetaSelectorLabels returns the selector labels for the Meta Deployment.
func MetaSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "meta",
		"app.kubernetes.io/component": "meta",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// RestLabels returns the common labels for the Rest component.
func RestLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "rest",
		"app.kubernetes.io/component":  "rest",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// RestSelectorLabels returns the selector labels for the Rest Deployment.
func RestSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "rest",
		"app.kubernetes.io/component": "rest",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// RealtimeLabels returns the common labels for the Realtime component.
func RealtimeLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "realtime",
		"app.kubernetes.io/component":  "realtime",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// RealtimeSelectorLabels returns the selector labels for the Realtime Deployment.
func RealtimeSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "realtime",
		"app.kubernetes.io/component": "realtime",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// AuthLabels returns the common labels for the Auth component.
func AuthLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "auth",
		"app.kubernetes.io/component":  "auth",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// AuthSelectorLabels returns the selector labels for the Auth Deployment.
func AuthSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "auth",
		"app.kubernetes.io/component": "auth",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// FunctionsLabels returns the common labels for the Functions component.
func FunctionsLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "functions",
		"app.kubernetes.io/component":  "functions",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// FunctionsSelectorLabels returns the selector labels for the Functions Deployment.
func FunctionsSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "functions",
		"app.kubernetes.io/component": "functions",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// EnvoyLabels returns the common labels for the Envoy component.
func EnvoyLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "envoy",
		"app.kubernetes.io/component":  "gateway",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// EnvoySelectorLabels returns the selector labels for the Envoy Deployment.
func EnvoySelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "envoy",
		"app.kubernetes.io/component": "gateway",
		"app.kubernetes.io/instance":  project.Name,
	}
}

// StudioLabels returns the common labels for the Studio component.
func StudioLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "studio",
		"app.kubernetes.io/component":  "studio",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// StudioSelectorLabels returns the selector labels for the Studio StatefulSet.
func StudioSelectorLabels(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "studio",
		"app.kubernetes.io/component": "studio",
		"app.kubernetes.io/instance":  project.Name,
	}
}
