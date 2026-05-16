package controller

import (
	"fmt"
	"strings"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	httpScheme  = "http"
	httpsScheme = "https"
)

func effectiveProtocol(httpConfig platformv1alpha1.HTTPConfig) string {
	if strings.EqualFold(httpConfig.Protocol, httpsScheme) {
		return httpsScheme
	}
	return httpScheme
}

func effectivePort(httpConfig platformv1alpha1.HTTPConfig, scheme string) int32 {
	if httpConfig.Port != nil {
		return *httpConfig.Port
	}
	if scheme == httpsScheme {
		return 443
	}
	return 80
}

func buildExternalURL(httpConfig platformv1alpha1.HTTPConfig) string {
	scheme := effectiveProtocol(httpConfig)
	port := effectivePort(httpConfig, scheme)

	if (scheme == httpScheme && port == 80) || (scheme == httpsScheme && port == 443) {
		return fmt.Sprintf("%s://%s", scheme, httpConfig.Hostname)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, httpConfig.Hostname, port)
}

func supabasePublicURL(project *platformv1alpha1.Project) string {
	return buildExternalURL(project.Spec.HTTP.API)
}

func storagePublicURL(project *platformv1alpha1.Project) string {
	return supabasePublicURL(project)
}

// PublicURL returns the public-facing base URL for the project.
func PublicURL(project *platformv1alpha1.Project) string {
	return supabasePublicURL(project)
}

// InternalURL returns the cluster-internal URL for the project.
// Deprecated: without a gateway, the public URL is used as the internal URL.
func InternalURL(project *platformv1alpha1.Project) string {
	return supabasePublicURL(project)
}

// StoragePublicURL returns the public URL for the storage API.
func StoragePublicURL(project *platformv1alpha1.Project) string {
	return storagePublicURL(project)
}
