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

func effectiveProtocol(httpSpec platformv1alpha1.HTTPSpec) string {
	if strings.EqualFold(httpSpec.Protocol, httpsScheme) {
		return httpsScheme
	}
	return httpScheme
}

func effectivePort(httpSpec platformv1alpha1.HTTPSpec, scheme string) int32 {
	if httpSpec.Port != nil {
		return *httpSpec.Port
	}
	if scheme == httpsScheme {
		return 443
	}
	return 80
}

func buildExternalURL(httpSpec platformv1alpha1.HTTPSpec) string {
	scheme := effectiveProtocol(httpSpec)
	port := effectivePort(httpSpec, scheme)

	if (scheme == httpScheme && port == 80) || (scheme == httpsScheme && port == 443) {
		return fmt.Sprintf("%s://%s", scheme, httpSpec.Hostname)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, httpSpec.Hostname, port)
}

func supabasePublicURL(project *platformv1alpha1.Project) string {
	return buildExternalURL(project.Spec.HTTP)
}

func supabaseInternalURL(project *platformv1alpha1.Project) string {
	internalHTTPSpec := project.Spec.HTTP
	internalHTTPSpec.Hostname = fmt.Sprintf("%s.%s.svc.cluster.local", project.Spec.HTTP.GatewayRef.Name, project.Spec.HTTP.GatewayRef.Namespace)
	return buildExternalURL(internalHTTPSpec)
}

func storagePublicURL(project *platformv1alpha1.Project) string {
	return fmt.Sprintf("%s/storage/v1", supabasePublicURL(project))
}

// PublicURL returns the public-facing base URL for the project.
func PublicURL(project *platformv1alpha1.Project) string {
	return supabasePublicURL(project)
}

// InternalURL returns the cluster-internal URL for the project.
func InternalURL(project *platformv1alpha1.Project) string {
	return supabaseInternalURL(project)
}

// StoragePublicURL returns the public URL for the storage API.
func StoragePublicURL(project *platformv1alpha1.Project) string {
	return storagePublicURL(project)
}
