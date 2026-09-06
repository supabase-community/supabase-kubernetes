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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
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

func TestOverlayServiceMergesTemplateAndKeepsManagedIdentity(t *testing.T) {
	const appName = "auth"

	base := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-service",
			Namespace: "managed-namespace",
			Labels:    map[string]string{"app": appName},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": appName},
			Ports: []corev1.ServicePort{{
				Name:       appName,
				Port:       9999,
				TargetPort: intstr.FromInt32(9999),
			}},
		},
	}
	user := &supabasev1alpha1.ServiceTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ignored",
			Namespace:   "ignored",
			Labels:      map[string]string{"team": "platform"},
			Annotations: map[string]string{"example.com/internal": "true"},
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
		},
	}

	merged, err := OverlayService(base, user)
	if err != nil {
		t.Fatalf("OverlayService() error = %v", err)
	}
	if merged.Name != "managed-service" || merged.Namespace != "managed-namespace" {
		t.Fatalf("managed identity = %s/%s", merged.Namespace, merged.Name)
	}
	if merged.Labels["app"] != appName || merged.Labels["team"] != "platform" {
		t.Fatalf("labels = %#v", merged.Labels)
	}
	if merged.Annotations["example.com/internal"] != "true" {
		t.Fatalf("annotations = %#v", merged.Annotations)
	}
	if merged.Spec.Type != corev1.ServiceTypeLoadBalancer || merged.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Fatalf("spec = %#v", merged.Spec)
	}
	if merged.Spec.Selector["app"] != appName || len(merged.Spec.Ports) != 1 || merged.Spec.Ports[0].Name != appName {
		t.Fatalf("default routing was not preserved: %#v", merged.Spec)
	}
}

func TestOverlayServiceStrategicallyMergesPorts(t *testing.T) {
	base := corev1.Service{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{
		Name:       "http",
		Port:       8000,
		TargetPort: intstr.FromInt32(8000),
	}}}}
	user := &supabasev1alpha1.ServiceTemplate{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
		{Name: "http", Port: 8000, TargetPort: intstr.FromInt32(8080)},
		{Name: "metrics", Port: 9090, TargetPort: intstr.FromInt32(9090)},
	}}}

	merged, err := OverlayService(base, user)
	if err != nil {
		t.Fatalf("OverlayService() error = %v", err)
	}
	if len(merged.Spec.Ports) != 2 {
		t.Fatalf("ports = %#v", merged.Spec.Ports)
	}
	if merged.Spec.Ports[0].Port != 8000 || merged.Spec.Ports[0].TargetPort != intstr.FromInt32(8080) {
		t.Fatalf("merged port = %#v", merged.Spec.Ports[0])
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

func TestOverlayPersistentVolumeClaimSpecMergesUserConfiguration(t *testing.T) {
	base := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("1Gi"),
		}},
	}
	className := "fast"
	user := &corev1.PersistentVolumeClaimSpec{
		StorageClassName: &className,
		Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("5Gi"),
		}},
	}

	merged, err := OverlayPersistentVolumeClaimSpec(base, user)
	if err != nil {
		t.Fatalf("OverlayPersistentVolumeClaimSpec() error = %v", err)
	}
	if len(merged.AccessModes) != 1 || merged.AccessModes[0] != corev1.ReadWriteOnce {
		t.Fatalf("accessModes = %#v", merged.AccessModes)
	}
	if merged.StorageClassName == nil || *merged.StorageClassName != className {
		t.Fatalf("storageClassName = %#v", merged.StorageClassName)
	}
	if merged.Resources.Requests.Storage().Cmp(resource.MustParse("5Gi")) != 0 {
		t.Fatalf("storage request = %s", merged.Resources.Requests.Storage())
	}
}
