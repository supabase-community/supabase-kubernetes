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

package function

import (
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// FunctionConfigMapName returns the name of the ConfigMap that holds the function source.
func FunctionConfigMapName(function *supabasev1alpha1.Function) string {
	return fmt.Sprintf("%s-function", function.Name)
}

// FunctionConfigMap constructs the ConfigMap containing the function source files.
func FunctionConfigMap(function *supabasev1alpha1.Function) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      FunctionConfigMapName(function),
			Namespace: function.Namespace,
			Labels:    FunctionLabels(function),
		},
		Data: maps.Clone(function.Spec.Source),
	}

	return cm, nil
}
