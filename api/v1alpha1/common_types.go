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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// PVCDeletionPolicy defines the deletion behavior for the PVC.
// +kubebuilder:validation:Enum=Delete;Retain
type PVCDeletionPolicy string

const (
	PVCDeletionPolicyDelete PVCDeletionPolicy = "Delete"
	PVCDeletionPolicyRetain PVCDeletionPolicy = "Retain"
)

// SecretKeyRef is a reference to a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ComponentProbes defines custom probe overrides for a component.
type ComponentProbes struct {
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
}

// VolumeClaimTemplateSpec defines the desired characteristics of a persistent volume claim.
type VolumeClaimTemplateSpec struct {
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +kubebuilder:validation:Required
	Resources corev1.VolumeResourceRequirements `json:"resources"`
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy PVCDeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ServiceSpec defines the configuration for a component Service.
type ServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HTTPConfig defines public HTTP access settings for a Project.
type HTTPConfig struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=http;https
	Protocol string `json:"protocol"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// WorkloadConfig defines common workload configuration shared by all Supabase components.
type WorkloadConfig struct {
	// image overrides the default component image
	// +optional
	Image string `json:"image,omitempty"`

	// imagePullPolicy defines the policy for if/when to pull the container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// replicas defines the number of component instances
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// affinity defines affinity scheduling rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// containerSecurityContext holds security configuration that will be applied to the container
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// nodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// podAnnotations defines annotations to add to the pod
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// podLabels defines labels to add to the pod
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// podSecurityContext holds pod-level security attributes
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// priorityClassName defines the priority class for the pod
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// probes defines custom probe overrides for the component
	// +optional
	Probes *ComponentProbes `json:"probes,omitempty"`

	// resources defines compute resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// service defines the configuration for the component Service
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// terminationGracePeriodSeconds defines the grace period for pod termination
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// env defines additional environment variables for the component container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// WorkloadJobConfig defines common workload configuration for Job-based resources.
type WorkloadJobConfig struct {
	// image overrides the default component image
	// +optional
	Image string `json:"image,omitempty"`

	// imagePullPolicy defines the policy for if/when to pull the container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// affinity defines affinity scheduling rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// containerSecurityContext holds security configuration that will be applied to the container
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// nodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// podAnnotations defines annotations to add to the pod
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// podLabels defines labels to add to the pod
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// podSecurityContext holds pod-level security attributes
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// priorityClassName defines the priority class for the pod
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// resources defines compute resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// terminationGracePeriodSeconds defines the grace period for pod termination
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// env defines additional environment variables for the component container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// DatabaseRef references a database resource.
type DatabaseRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=SingleDatabase
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// RestRef references a Rest resource.
type RestRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Rest
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// AuthRef references an Auth resource.
type AuthRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Auth
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// MetaRef references a Meta resource.
type MetaRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Meta
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// RealtimeRef references a Realtime resource.
type RealtimeRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Realtime
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}
