//go:build integration

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

package assets

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Run with: go test -tags=integration ./internal/assets -run TestReadOnlyPasswordSync -v
// All database changes are confined to a disposable, network-isolated container.
func TestReadOnlyPasswordSync(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatal("Docker is required for the password sync integration test")
	}
	run := func(input string, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stdin = strings.NewReader(input)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("docker command failed: %v\n%s\n%s", err, out, stderr.String())
		}
		return strings.TrimSpace(string(out))
	}
	container := run("", "run", "--detach", "--rm", "--network=none",
		"-e", "POSTGRES_PASSWORD=integration-admin-password",
		"-e", "POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256", "postgres:17-alpine")
	t.Cleanup(func() {
		if out, err := exec.Command("docker", "rm", "--force", container).CombinedOutput(); err != nil {
			t.Errorf("remove test container: %v: %s", err, out)
		}
	})
	deadline := time.Now().Add(time.Minute)
	for exec.Command("docker", "exec", container, "pg_isready", "-U", "postgres").Run() != nil {
		if time.Now().After(deadline) {
			t.Fatal("test PostgreSQL did not become ready")
		}
		time.Sleep(time.Second)
	}
	run("CREATE DATABASE _supabase;\n", "exec", "-i", container,
		"psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "postgres")
	fixture := `DO $$ DECLARE role_name text; BEGIN
FOR role_name IN SELECT unnest(ARRAY['anon','authenticated','authenticator',
'dashboard_user','pgbouncer','service_role','supabase_admin','supabase_auth_admin',
'supabase_functions_admin','supabase_replication_admin','supabase_storage_admin']) LOOP
EXECUTE format('CREATE ROLE %I', role_name);
END LOOP; END $$;
CREATE ROLE supabase_read_only_user LOGIN BYPASSRLS;
GRANT pg_read_all_data TO supabase_read_only_user;
ALTER ROLE supabase_read_only_user SET default_transaction_read_only=on;
CREATE SCHEMA _supavisor;
`
	run(fixture, "exec", "-i", container, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "_supabase")
	for _, password := range []string{"first'password", "rotated'password"} {
		run(ProjectSyncPasswordScript, "exec", "-i",
			"-e", "PGHOST=/var/run/postgresql", "-e", "PGPORT=5432", "-e", "PGUSER=postgres",
			"-e", "PGPASSWORD="+password, "-e", "DB_ADMIN_USER=postgres",
			"-e", "DB_SRV_NAME=localhost", "-e", "DB_SRV_PORT=5432", container, "sh")
		got := run("", "exec", "-e", "PGPASSWORD="+password, container, "psql", "-X", "-w",
			"-h", "127.0.0.1", "-U", "supabase_read_only_user", "-d", "postgres", "-Atc",
			"SELECT current_user, current_setting('transaction_read_only'), rolsuper, rolbypassrls FROM pg_roles WHERE rolname=current_user")
		if got != "supabase_read_only_user|on|f|t" {
			t.Fatalf("unexpected read-only role state after sync: %s", got)
		}
	}
	cmd := exec.Command("docker", "exec", "-e", "PGPASSWORD=first'password", container,
		"psql", "-X", "-w", "-h", "127.0.0.1", "-U", "supabase_read_only_user", "-d", "postgres", "-Atc", "SELECT 1")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "password authentication failed") {
		t.Fatalf("old password was not rejected after rotation: %v: %s", err, out)
	}
}
