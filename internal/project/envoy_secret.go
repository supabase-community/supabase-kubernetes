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

package project

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// EnvoySecretName returns the name of the dashboard credentials Secret for a Project.
func EnvoySecretName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-envoy-auth", project.Name)
}

// EnvoySecret constructs the dashboard credentials Secret for a Project.
func EnvoySecret(project *supabasev1alpha1.Project) (*corev1.Secret, error) {
	password, err := helper.GenerateRandomAlphanumeric(32)
	if err != nil {
		return nil, fmt.Errorf("generating envoy dashboard password: %w", err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EnvoySecretName(project),
			Namespace: project.Namespace,
			Labels:    EnvoyLabels(project),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			DefaultEnvoySecretKeyUsername: []byte("supabase"),
			DefaultEnvoySecretKeyPassword: []byte(password),
		},
	}, nil
}
