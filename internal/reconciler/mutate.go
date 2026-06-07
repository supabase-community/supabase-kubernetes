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

package reconciler

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// MutateSecret returns a mutateFn that preserves existing keys and copies
// missing ones from desired for the provided key names.
func MutateSecret(keys ...string) func(existing, desired *corev1.Secret) error {
	return func(existing, desired *corev1.Secret) error {
		for _, key := range keys {
			if _, ok := existing.Data[key]; ok {
				continue
			}
			existing.Data[key] = desired.Data[key]
		}
		return nil
	}
}

// MutatePVC returns a mutateFn that copies Resources from desired to existing.
func MutatePVC() func(existing, desired *corev1.PersistentVolumeClaim) error {
	return func(existing, desired *corev1.PersistentVolumeClaim) error {
		existing.Spec.Resources = desired.Spec.Resources
		return nil
	}
}

// MutateService returns a mutateFn that copies Spec, Labels and Annotations
// from desired to existing.
func MutateService() func(existing, desired *corev1.Service) error {
	return func(existing, desired *corev1.Service) error {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		return nil
	}
}

// MutateConfigMap returns a mutateFn that copies Data, Labels and Annotations
// from desired to existing.
func MutateConfigMap() func(existing, desired *corev1.ConfigMap) error {
	return func(existing, desired *corev1.ConfigMap) error {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		return nil
	}
}

// MutateStatefulSet returns a mutateFn that copies Spec, Labels and
// Annotations from desired to existing.
func MutateStatefulSet() func(existing, desired *appsv1.StatefulSet) error {
	return func(existing, desired *appsv1.StatefulSet) error {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		return nil
	}
}

// MutateDeployment returns a mutateFn that copies Spec, Labels and
// Annotations from desired to existing.
func MutateDeployment() func(existing, desired *appsv1.Deployment) error {
	return func(existing, desired *appsv1.Deployment) error {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		return nil
	}
}
