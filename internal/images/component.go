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

package images

// images maps a Supabase version to component images.
// Each version must explicitly declare all supported component images.
var images = map[string]map[string]string{
	"2026.04.27": {
		ComponentRest:      "postgrest/postgrest:v14.8",
		ComponentMeta:      "supabase/postgres-meta:v0.96.3",
		ComponentRealtime:  "supabase/realtime:v2.76.5",
		ComponentAuth:      "supabase/gotrue:v2.186.0",
		ComponentDatabase:  "supabase/postgres:17.6.1.084",
		ComponentMigration: "supabase/postgres:17.6.1.084",
	},
}

// Resolve returns the container image for a given version and component.
// If the version is not registered, it falls back to DefaultVersion.
// If the component is not registered, it returns an empty string.
func Resolve(version, component string) string {
	components, ok := images[version]
	if !ok {
		components = images[DefaultVersion]
	}
	image, ok := components[component]
	if !ok {
		return ""
	}
	return image
}
