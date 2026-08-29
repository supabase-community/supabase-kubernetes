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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretKeyRef is a reference to a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// Name defines the name of the Kubernetes Secret
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Key defines the key within the Kubernetes Secret
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Key string `json:"key"`
}

// ServiceTemplate defines a template for a Kubernetes Service managed by the operator.
type ServiceTemplate struct {
	// ObjectMeta is the metadata of the service.
	// The name and namespace provided here are managed by the operator and will be ignored.
	// +kubebuilder:validation:Optional
	ObjectMeta metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the service.
	// +kubebuilder:validation:Optional
	Spec corev1.ServiceSpec `json:"spec,omitempty"`
}

// DatabaseReference references a database resource.
type DatabaseReference struct {
	// Kind defines the kind of database resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Enum=SingleDatabase
	Kind string `json:"kind"`

	// Name defines the name of the database resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}
