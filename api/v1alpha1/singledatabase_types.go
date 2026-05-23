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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCDeletionPolicy defines the deletion behavior for the PVC.
// +kubebuilder:validation:Enum=Delete;Retain
type PVCDeletionPolicy string

const (
	PVCDeletionPolicyDelete PVCDeletionPolicy = "Delete"
	PVCDeletionPolicyRetain PVCDeletionPolicy = "Retain"
)

// SingleDatabaseServiceSpec configures the Service created for the PostgreSQL instance.
type SingleDatabaseServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// SingleDatabaseSpec defines the desired state of SingleDatabase.
type SingleDatabaseSpec struct {
	Image string `json:"image"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Storage VolumeClaimTemplateSpec `json:"storage,omitempty"`
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:default=Delete
	// +optional
	PVCDeletionPolicy PVCDeletionPolicy `json:"pvcDeletionPolicy,omitempty"`
	// +optional
	Service *SingleDatabaseServiceSpec `json:"service,omitempty"`
	// +optional
	Probes *ComponentProbes `json:"probes,omitempty"`
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// SingleDatabaseStatus defines the observed state of SingleDatabase.
type SingleDatabaseStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`
	// +optional
	Storage string `json:"storage,omitempty"`
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	ServiceName string `json:"serviceName,omitempty"`
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=singledatabases,scope=Namespaced
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Storage",type=string,JSONPath=`.status.storage`
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.status.serviceName`
// +kubebuilder:printcolumn:name="Secret",type=string,JSONPath=`.status.secretName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SingleDatabase is the Schema for the singledatabases API.
type SingleDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SingleDatabaseSpec   `json:"spec,omitempty"`
	Status            SingleDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SingleDatabaseList contains a list of SingleDatabase.
type SingleDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SingleDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SingleDatabase{}, &SingleDatabaseList{})
}
