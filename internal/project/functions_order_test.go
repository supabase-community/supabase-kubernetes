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
	"reflect"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func testFunction(name string, files ...string) supabasev1alpha1.Function {
	source := make(map[string]string, len(files))
	for _, file := range files {
		source[file] = "export {};"
	}
	return supabasev1alpha1.Function{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       supabasev1alpha1.FunctionSpec{FunctionName: name, Source: source},
	}
}

// The cache lists functions in no particular order and Go ranges over a map in
// random order, so the builders must render the same pod template from any input
// order; otherwise every reconcile rolls the Functions Deployment and the Studio
// StatefulSet again.
func TestFunctionVolumesAndMountsAreStable(t *testing.T) {
	router := testFunction("p-main", "index.ts")
	app := testFunction("p-app", "index.ts", "apply.ts", "auth.ts", "brands.js", "bunny.ts",
		"db.ts", "email.ts", "ghl.ts", "validate.js", "version.js")
	forward := []supabasev1alpha1.Function{router, app}
	reverse := []supabasev1alpha1.Function{app, router}

	wantMounts := buildFunctionsVolumeMounts(forward)
	wantVolumes := buildFunctionsVolumes(forward)
	wantStudioMounts := buildStudioVolumeMounts(forward)
	wantStudioVolumes := buildStudioFunctionVolumes(forward)

	for run := range 100 {
		in := forward
		if run%2 == 1 {
			in = reverse
		}
		if got := buildFunctionsVolumeMounts(in); !reflect.DeepEqual(got, wantMounts) {
			t.Fatalf("run %d: functions mounts changed", run)
		}
		if got := buildFunctionsVolumes(in); !reflect.DeepEqual(got, wantVolumes) {
			t.Fatalf("run %d: functions volumes changed", run)
		}
		if got := buildStudioVolumeMounts(in); !reflect.DeepEqual(got, wantStudioMounts) {
			t.Fatalf("run %d: studio mounts changed", run)
		}
		if got := buildStudioFunctionVolumes(in); !reflect.DeepEqual(got, wantStudioVolumes) {
			t.Fatalf("run %d: studio volumes changed", run)
		}
	}

	paths := make([]string, 0, len(wantMounts))
	for _, m := range wantMounts {
		paths = append(paths, m.MountPath)
	}
	if !slices.IsSorted(paths) || len(paths) != 11 {
		t.Fatalf("functions mounts not sorted by function and file: %v", paths)
	}
	if reverse[0].Name != "p-app" {
		t.Fatalf("the input slice was reordered")
	}
}
