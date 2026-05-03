package controller

import (
	"fmt"
	"sort"
	"strings"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func resolveComponentImage(project *platformv1alpha1.Project, componentName, overrideImage string) (string, error) {
	if strings.TrimSpace(overrideImage) != "" {
		return overrideImage, nil
	}

	version := strings.TrimSpace(project.Spec.Version)
	if version == "" {
		return "", fmt.Errorf("project spec.version is required to resolve image for component %q", componentName)
	}

	componentImages, ok := projectVersionCatalog[version]
	if !ok {
		return "", fmt.Errorf("unsupported project version %q, supported versions: %s", version, strings.Join(supportedProjectVersions(), ", "))
	}

	image, ok := componentImages[componentName]
	if !ok || strings.TrimSpace(image) == "" {
		return "", fmt.Errorf("catalog for version %q does not define image for component %q", version, componentName)
	}

	return image, nil
}

func supportedProjectVersions() []string {
	versions := make([]string, 0, len(projectVersionCatalog))
	for version := range projectVersionCatalog {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions
}
