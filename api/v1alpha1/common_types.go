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

// ResolvedDatabase exposes resolved database connection parameters.
type ResolvedDatabase struct {
	// Host defines the database host
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Port defines the database port
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// DBName defines the database name
	// +kubebuilder:validation:Required
	DBName string `json:"dbName"`

	// User defines the database user
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// PasswordRef references the secret containing the database password
	// +kubebuilder:validation:Required
	PasswordRef SecretKeyRef `json:"passwordRef"`
}

// ServiceTemplate defines the template for a component Service.
type ServiceTemplate struct {
	// ObjectMeta defines metadata overlaid on the operator-generated Service.
	// Name and namespace are managed by the operator and are ignored.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	ObjectMeta metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the Kubernetes Service specification overlaid on the
	// operator-generated Service specification.
	// +optional
	Spec corev1.ServiceSpec `json:"spec,omitempty"`
}

// DatabaseRef references a database resource.
type DatabaseRef struct {
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
