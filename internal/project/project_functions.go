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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
)

// ProjectMainFunctionName returns the name of the default main Function for a Project.
func ProjectMainFunctionName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-main", project.Name)
}

// ProjectMainFunction constructs the default main Function for a Project.
func ProjectMainFunction(project *supabasev1alpha1.Project) (*supabasev1alpha1.Function, error) {
	if project.Spec.Functions == nil || !*project.Spec.Functions.Enable {
		return nil, nil
	}

	return &supabasev1alpha1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProjectMainFunctionName(project),
			Namespace: project.Namespace,
			Labels:    ProjectLabels(project),
		},
		Spec: supabasev1alpha1.FunctionSpec{
			ProjectRef:   project.Name,
			FunctionName: "main",
			Source: map[string]string{
				"index.ts": assets.MainFunction,
			},
		},
	}, nil
}
