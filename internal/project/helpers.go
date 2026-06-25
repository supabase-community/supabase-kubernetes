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
	"crypto/sha256"
	"fmt"
	"strconv"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// ComputePasswordSyncHash returns a stable hash of the password configuration
// that the sync-password job applies. It is used to decide whether the job
// needs to run again.
func ComputePasswordSyncHash(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, dbPassword string) string {
	h := sha256.New()
	h.Write([]byte(db.User))
	h.Write([]byte("\x00"))
	h.Write([]byte(db.Host))
	h.Write([]byte("\x00"))
	h.Write([]byte(strconv.Itoa(int(db.Port))))
	h.Write([]byte("\x00"))
	h.Write([]byte(dbPassword))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ComputeJWTSyncHash returns a stable hash of the JWT configuration that the
// sync-jwt job applies. It is used to decide whether the job needs to run again.
func ComputeJWTSyncHash(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, dbPassword, jwtSecret string) string {
	h := sha256.New()
	h.Write([]byte(ComputePasswordSyncHash(project, db, dbPassword)))
	h.Write([]byte("\x00"))
	h.Write([]byte(jwtSecret))
	h.Write([]byte("\x00"))
	h.Write([]byte(strconv.Itoa(int(*project.Spec.JWTExpSec))))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// APIExternalURL returns the public API URL for a Project.
func APIExternalURL(project *supabasev1alpha1.Project) string {
	url := fmt.Sprintf("%s://%s", project.Spec.HTTP.Protocol, project.Spec.HTTP.Hostname)
	if project.Spec.HTTP.Port != nil {
		url = fmt.Sprintf("%s:%d", url, *project.Spec.HTTP.Port)
	}
	return url
}
