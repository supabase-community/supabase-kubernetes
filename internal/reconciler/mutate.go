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
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// mergeStringMaps returns a new map containing all entries from existing
// overwritten by entries from desired. Nil is returned only when both inputs
// are nil.
func mergeStringMaps(existing, desired map[string]string) map[string]string {
	if existing == nil && desired == nil {
		return nil
	}
	merged := make(map[string]string, len(existing)+len(desired))
	maps.Copy(merged, existing)
	maps.Copy(merged, desired)
	return merged
}

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

// MutateService returns a mutateFn that copies the managed Service fields
// (Type, Selector, Ports) and Labels from desired to existing, merging
// Annotations so that server-side or third-party annotations are preserved.
// Server-side defaults such as ClusterIP and ClusterIPs are preserved.
func MutateService() func(existing, desired *corev1.Service) error {
	return func(existing, desired *corev1.Service) error {
		existing.Spec.Type = desired.Spec.Type
		existing.Spec.Selector = desired.Spec.Selector
		existing.Spec.Ports = desired.Spec.Ports
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}

// MutateConfigMap returns a mutateFn that copies Data and Labels from desired
// to existing, merging Annotations so that existing entries are preserved.
func MutateConfigMap() func(existing, desired *corev1.ConfigMap) error {
	return func(existing, desired *corev1.ConfigMap) error {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}

// MutateStatefulSet returns a mutateFn that copies the managed StatefulSet
// fields from desired to existing, preserving server-side defaults.
func MutateStatefulSet() func(existing, desired *appsv1.StatefulSet) error {
	return func(existing, desired *appsv1.StatefulSet) error {
		existing.Spec.Replicas = desired.Spec.Replicas
		existing.Spec.Selector = desired.Spec.Selector
		mergePodTemplateSpec(&existing.Spec.Template, &desired.Spec.Template)
		existing.Spec.VolumeClaimTemplates = desired.Spec.VolumeClaimTemplates
		existing.Spec.ServiceName = desired.Spec.ServiceName
		existing.Spec.PodManagementPolicy = desired.Spec.PodManagementPolicy
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}

// MutateDeployment returns a mutateFn that copies the managed Deployment
// fields from desired to existing, preserving server-side defaults such as
// strategy, progressDeadlineSeconds and pod spec defaults.
func MutateDeployment() func(existing, desired *appsv1.Deployment) error {
	return func(existing, desired *appsv1.Deployment) error {
		existing.Spec.Replicas = desired.Spec.Replicas
		existing.Spec.Selector = desired.Spec.Selector
		mergePodTemplateSpec(&existing.Spec.Template, &desired.Spec.Template)
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}

// mergePodTemplateSpec copies the fields of the PodTemplateSpec that the
// operator manages while preserving API-server defaults (e.g. DNSPolicy,
// RestartPolicy, schedulerName, terminationMessagePath) on the existing object.
func mergePodTemplateSpec(existing, desired *corev1.PodTemplateSpec) {
	existing.ObjectMeta = desired.ObjectMeta
	mergePodSpec(&existing.Spec, &desired.Spec)
}

// mergePodSpec copies the operator-managed fields from desired to existing,
// preserving server-side defaults on fields the operator does not set.
func mergePodSpec(existing, desired *corev1.PodSpec) {
	existing.Affinity = desired.Affinity
	existing.NodeSelector = desired.NodeSelector
	existing.Tolerations = desired.Tolerations
	existing.PriorityClassName = desired.PriorityClassName
	existing.SecurityContext = desired.SecurityContext
	existing.TerminationGracePeriodSeconds = desired.TerminationGracePeriodSeconds
	existing.Volumes = desired.Volumes
	existing.ServiceAccountName = desired.ServiceAccountName
	existing.ImagePullSecrets = desired.ImagePullSecrets
	existing.HostAliases = desired.HostAliases
	existing.InitContainers = mergeContainers(existing.InitContainers, desired.InitContainers)
	existing.Containers = mergeContainers(existing.Containers, desired.Containers)
}

// mergeContainers replaces containers by name, copying operator-managed fields
// while preserving API-server defaults such as terminationMessagePath and
// probe successThreshold.
func mergeContainers(existing, desired []corev1.Container) []corev1.Container {
	if len(desired) == 0 {
		return existing
	}

	result := make([]corev1.Container, len(desired))
	existingByName := make(map[string]corev1.Container, len(existing))
	for _, c := range existing {
		existingByName[c.Name] = c
	}

	for i, d := range desired {
		e, ok := existingByName[d.Name]
		if !ok {
			e = corev1.Container{Name: d.Name}
		}
		mergeContainer(&e, &d)
		result[i] = e
	}
	return result
}

// mergeContainer copies operator-managed fields from desired to existing,
// preserving server-side defaults.
func mergeContainer(existing, desired *corev1.Container) {
	existing.Name = desired.Name
	existing.Image = desired.Image
	existing.Command = desired.Command
	existing.Args = desired.Args
	existing.WorkingDir = desired.WorkingDir
	existing.Ports = desired.Ports
	existing.EnvFrom = desired.EnvFrom
	existing.Env = desired.Env
	existing.Resources = desired.Resources
	existing.VolumeMounts = desired.VolumeMounts
	existing.VolumeDevices = desired.VolumeDevices
	existing.Lifecycle = desired.Lifecycle
	existing.ImagePullPolicy = desired.ImagePullPolicy
	existing.SecurityContext = desired.SecurityContext

	existing.LivenessProbe = mergeProbe(existing.LivenessProbe, desired.LivenessProbe)
	existing.ReadinessProbe = mergeProbe(existing.ReadinessProbe, desired.ReadinessProbe)
	existing.StartupProbe = mergeProbe(existing.StartupProbe, desired.StartupProbe)

	// Preserve server-side defaults when the operator does not set them.
	if desired.TerminationMessagePath == "" && existing.TerminationMessagePath != "" {
		// keep existing default
	} else {
		existing.TerminationMessagePath = desired.TerminationMessagePath
	}
	if desired.TerminationMessagePolicy == "" && existing.TerminationMessagePolicy != "" {
		// keep existing default
	} else {
		existing.TerminationMessagePolicy = desired.TerminationMessagePolicy
	}
}

// mergeProbe copies the operator-managed probe fields from desired to existing,
// preserving the API-server default for SuccessThreshold.
func mergeProbe(existing, desired *corev1.Probe) *corev1.Probe {
	if desired == nil {
		return existing
	}
	if existing == nil {
		cp := desired.DeepCopy()
		return cp
	}
	existing.ProbeHandler = desired.ProbeHandler
	existing.InitialDelaySeconds = desired.InitialDelaySeconds
	existing.TerminationGracePeriodSeconds = desired.TerminationGracePeriodSeconds
	existing.PeriodSeconds = desired.PeriodSeconds
	existing.TimeoutSeconds = desired.TimeoutSeconds
	existing.FailureThreshold = desired.FailureThreshold
	if desired.SuccessThreshold != 0 {
		existing.SuccessThreshold = desired.SuccessThreshold
	}
	return existing
}

// MutateJob returns a mutateFn that copies Spec and Labels from desired to
// existing, merging Annotations so that existing entries are preserved. The
// generated selector and template labels are preserved because they are
// injected by the API server and must not be overwritten.
func MutateJob() func(existing, desired *batchv1.Job) error {
	return func(existing, desired *batchv1.Job) error {
		selector := existing.Spec.Selector.DeepCopy()

		// Merge template labels so auto-generated keys
		// (controller-uid, job-name, etc.) are preserved.
		templateLabels := make(map[string]string, len(existing.Spec.Template.Labels)+len(desired.Spec.Template.Labels))
		maps.Copy(templateLabels, existing.Spec.Template.Labels)
		maps.Copy(templateLabels, desired.Spec.Template.Labels)

		existing.Spec = desired.Spec
		existing.Spec.Selector = selector
		existing.Spec.Template.Labels = templateLabels
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}

// MutateMigration returns a mutateFn that copies Spec and Labels from desired
// to existing, merging Annotations so that existing entries are preserved.
func MutateMigration() func(existing, desired *supabasev1alpha1.Migration) error {
	return func(existing, desired *supabasev1alpha1.Migration) error {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}

// MutateFunction returns a mutateFn that copies Spec and Labels from desired
// to existing, merging Annotations so that existing entries are preserved.
func MutateFunction() func(existing, desired *supabasev1alpha1.Function) error {
	return func(existing, desired *supabasev1alpha1.Function) error {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		existing.Annotations = mergeStringMaps(existing.Annotations, desired.Annotations)
		return nil
	}
}
