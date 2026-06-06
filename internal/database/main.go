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

package database

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	// DefaultPort is the default PostgreSQL port.
	DefaultPort = int32(5432)
	// DefaultDBName is the default PostgreSQL database name.
	DefaultDBName = "postgres"
	// DefaultSecretKey is the default secret key holding the database password.
	DefaultSecretKey = "password"
	// DefaultUser is the default database user.
	DefaultUser = "supabase_admin"
)

// ResolvedDatabase holds resolved connection parameters for a DatabaseRef.
type ResolvedDatabase struct {
	Host              string
	Port              int32
	DBName            string
	User              string
	SecretName        string
	SecretPasswordKey string
}

// ResolveRef resolves a DatabaseRef into connection parameters.
// It returns the resolved database, a boolean indicating whether the database
// is ready, and an error if resolution itself failed (e.g. unsupported kind).
// A false ready value with a nil error means the referenced database was not
// found or is not ready yet; the caller should requeue.
func ResolveRef(
	ctx context.Context,
	c client.Client,
	ref supabasev1alpha1.DatabaseRef,
	namespace string,
) (*ResolvedDatabase, bool, error) {
	switch ref.Kind {
	case "SingleDatabase":
		return resolveSingleDatabase(ctx, c, ref, namespace)
	default:
		return nil, false, fmt.Errorf("unsupported database kind %q", ref.Kind)
	}
}
