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

package controller

import "fmt"

// versionedImages maps a Supabase version to component images.
// Each version must explicitly declare all supported component images.
var versionedImages = map[string]map[string]string{
	"2026.04.27": {
		"rest":      "postgrest/postgrest:v14.8",
		"meta":      "supabase/postgres-meta:v0.96.3",
		"realtime":  "supabase/realtime:v2.76.5",
		"auth":      "supabase/gotrue:v2.186.0",
		"database":  "supabase/postgres:17.6.1.084",
		"migration": "supabase/postgres:17.6.1.084",
	},
}

// ErrVersionNotSupported is returned when a version is not found in the image map.
type ErrVersionNotSupported struct {
	Version   string
	Component string
}

func (e *ErrVersionNotSupported) Error() string {
	return fmt.Sprintf("version %q does not have a registered image for component %q", e.Version, e.Component)
}

// ResolveComponentImage returns the container image for a given version and component.
// If the version or component is not registered, it returns an error.
func ResolveComponentImage(version, component string) (string, error) {
	components, ok := versionedImages[version]
	if !ok {
		return "", &ErrVersionNotSupported{Version: version, Component: component}
	}
	image, ok := components[component]
	if !ok {
		return "", &ErrVersionNotSupported{Version: version, Component: component}
	}
	return image, nil
}
