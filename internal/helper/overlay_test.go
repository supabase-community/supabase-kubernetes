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

package helper

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOverlayMergesMetadataAndNamedPodResources(t *testing.T) {
	base := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "base"}},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "base:v1",
				Env:   []corev1.EnvVar{{Name: "BASE", Value: "true"}},
			}},
			Volumes: []corev1.Volume{{Name: "data"}},
		},
	}
	user := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"team": "platform"}},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "custom:v1", Env: []corev1.EnvVar{{Name: "CUSTOM", Value: "true"}}},
				{Name: "sidecar", Image: "sidecar:v1"},
			},
			Volumes: []corev1.Volume{{Name: "cache"}},
		},
	}

	merged, err := Overlay(base, user)
	if err != nil {
		t.Fatalf("Overlay() error = %v", err)
	}
	if merged.Labels["app"] != "base" || merged.Labels["team"] != "platform" {
		t.Fatalf("labels = %#v", merged.Labels)
	}
	if len(merged.Spec.Containers) != 2 {
		t.Fatalf("containers = %#v", merged.Spec.Containers)
	}
	if merged.Spec.Containers[0].Name != "app" || merged.Spec.Containers[0].Image != "custom:v1" {
		t.Fatalf("main container = %#v", merged.Spec.Containers[0])
	}
	if merged.Spec.Containers[1].Name != "sidecar" || merged.Spec.Containers[1].Image != "sidecar:v1" {
		t.Fatalf("sidecar = %#v", merged.Spec.Containers[1])
	}
	if len(merged.Spec.Volumes) != 2 || merged.Spec.Volumes[0].Name != "cache" || merged.Spec.Volumes[1].Name != "data" {
		t.Fatalf("volumes = %#v", merged.Spec.Volumes)
	}
}

func TestOverlayKeepsBaseContainersWhenUserLeavesThemUnset(t *testing.T) {
	base := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "postgres", Image: "postgres:15"}},
		},
	}
	// Mirrors config/samples/singledatabase.yaml: only metadata and nodeSelector set.
	user := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"workload": "database"}},
		Spec:       corev1.PodSpec{NodeSelector: map[string]string{"workload": "database"}},
	}

	merged, err := Overlay(base, user)
	if err != nil {
		t.Fatalf("Overlay() error = %v", err)
	}
	if len(merged.Spec.Containers) != 1 || merged.Spec.Containers[0].Name != "postgres" {
		t.Fatalf("containers = %#v", merged.Spec.Containers)
	}
	if merged.Spec.NodeSelector["workload"] != "database" {
		t.Fatalf("nodeSelector = %#v", merged.Spec.NodeSelector)
	}
}
