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
	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// FunctionLabels returns the common labels for a Function and its resources.
func FunctionLabels(function *supabasev1alpha1.Function) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "function",
		"app.kubernetes.io/component":  "function",
		"app.kubernetes.io/instance":   function.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}
