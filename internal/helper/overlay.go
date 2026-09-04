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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// Overlay merges a user Pod template over an operator-generated base template.
// Kubernetes strategic-merge rules merge named lists, such as containers,
// volumes, and environment variables, by their merge key.
func Overlay(base corev1.PodTemplateSpec, user *corev1.PodTemplateSpec) (corev1.PodTemplateSpec, error) {
	if user == nil {
		return base, nil
	}

	baseJSON, err := json.Marshal(base)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	userJSON, err := json.Marshal(user)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	// Drop null values from the user patch: corev1 fields without omitempty
	// marshal as null, and a strategic merge patch treats explicit null as
	// delete, which would remove operator-managed fields.
	userJSON, err = omitNulls(userJSON)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	mergedJSON, err := strategicpatch.StrategicMergePatch(baseJSON, userJSON, corev1.PodTemplateSpec{})
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	var merged corev1.PodTemplateSpec
	if err := json.Unmarshal(mergedJSON, &merged); err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	return merged, nil
}

// omitNulls removes null values from raw JSON so they are not interpreted as
// deletions by the strategic merge patch.
func omitNulls(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return json.Marshal(pruneNulls(value))
}

func pruneNulls(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if item == nil {
				delete(typed, key)
				continue
			}
			typed[key] = pruneNulls(item)
		}
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = pruneNulls(item)
		}
		return typed
	default:
		return value
	}
}
