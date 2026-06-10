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
	"k8s.io/apimachinery/pkg/api/resource"
)

// VolumeClaim defines the desired characteristics of a persistent volume claim.
type VolumeClaim struct {
	// AccessModes defines the access modes for the persistent volume claim
	// +kubebuilder:validation:Required
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`

	// StorageClassName defines the storage class name for the persistent volume claim
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Size defines the size of the persistent volume claim
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`

	// DeletionPolicy defines the deletion behavior for the persistent volume claim
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// DeletionPolicy defines the deletion behavior for the PVC.
// +kubebuilder:validation:Enum=Delete;Retain
type DeletionPolicy string

const (
	DeletionPolicyDelete DeletionPolicy = "Delete"
	DeletionPolicyRetain DeletionPolicy = "Retain"
)

// SecretKeyRef is a reference to a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// Name defines the name of the Kubernetes Secret
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key defines the key within the Kubernetes Secret
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ResolvedDatabase exposes resolved database connection parameters.
type ResolvedDatabase struct {
	// Host defines the database host
	Host string `json:"host"`

	// Port defines the database port
	Port int32 `json:"port"`

	// DBName defines the database name
	DBName string `json:"dbName"`

	// User defines the database user
	User string `json:"user,omitempty"`

	// PasswordRef references the secret containing the database password
	PasswordRef SecretKeyRef `json:"passwordRef"`
}

// ServiceSpec defines the configuration for a component Service.
type ServiceSpec struct {
	// Type defines the type of the Kubernetes Service
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Annotations defines annotations to add to the Service
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels defines labels to add to the Service
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HTTPConfig defines public HTTP access settings for a Project.
type HTTPConfig struct {
	// Protocol defines the HTTP protocol (http or https)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=http;https
	Protocol string `json:"protocol"`

	// Hostname defines the public hostname for the Project
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`

	// Port defines the public port for the Project
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// WorkloadConfig defines common workload configuration shared by all Supabase components.
type WorkloadConfig struct {
	// Image overrides the default component image
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy defines the policy for if/when to pull the container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Affinity defines affinity scheduling rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// ContainerSecurityContext holds security configuration that will be applied to the container
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// NodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// PodAnnotations defines annotations to add to the pod
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// PodLabels defines labels to add to the pod
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// PodSecurityContext holds pod-level security attributes
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// PriorityClassName defines the priority class for the pod
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// Resources defines compute resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// TerminationGracePeriodSeconds defines the grace period for pod termination
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// Tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Env defines additional environment variables for the component container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// DatabaseRef references a database resource.
type DatabaseRef struct {
	// Kind defines the kind of database resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=SingleDatabase
	Kind string `json:"kind"`

	// Name defines the name of the database resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}
