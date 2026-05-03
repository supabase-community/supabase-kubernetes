package controller

import (
	"strings"
	"testing"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func TestResolveComponentImage_UsesOverride(t *testing.T) {
	project := &platformv1alpha1.Project{}

	image, err := resolveComponentImage(project, componentStudio, "custom/studio:tag")
	if err != nil {
		t.Fatalf("resolveComponentImage returned error: %v", err)
	}
	if image != "custom/studio:tag" {
		t.Fatalf("expected override image, got %q", image)
	}
}

func TestResolveComponentImage_UsesCatalog(t *testing.T) {
	project := &platformv1alpha1.Project{
		Spec: platformv1alpha1.ProjectSpec{Version: "2026.04.27"},
	}

	image, err := resolveComponentImage(project, componentStudio, "")
	if err != nil {
		t.Fatalf("resolveComponentImage returned error: %v", err)
	}
	if image != "supabase/studio:2026.04.27-sha-5f60601" {
		t.Fatalf("unexpected catalog image %q", image)
	}
}

func TestResolveComponentImage_RejectsUnsupportedVersion(t *testing.T) {
	project := &platformv1alpha1.Project{
		Spec: platformv1alpha1.ProjectSpec{Version: "2099.01.01"},
	}

	_, err := resolveComponentImage(project, componentStudio, "")
	if err == nil {
		t.Fatalf("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported project version") {
		t.Fatalf("unexpected error: %v", err)
	}
}
