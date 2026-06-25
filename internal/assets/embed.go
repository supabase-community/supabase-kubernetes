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

import "embed"

//go:embed migrations/*.sql
var MigrationFiles embed.FS

//go:embed migrations/supabase.sql
var SupabaseMigration string

//go:embed migrations/realtime.sql
var RealtimeMigration string

//go:embed migrations/logs.sql
var LogsMigration string

//go:embed migrations/pooler.sql
var PoolerMigration string

//go:embed migrations/webhooks.sql
var WebhooksMigration string

//go:embed scripts/migration-apply.sh
var MigrationApplyScript string

//go:embed scripts/singledatabase-password-sync.sh
var SingleDatabasePasswordSyncScript string

//go:embed scripts/singledatabase-pgsodium-init.sh
var SingleDatabasePgsodiumInitScript string

//go:embed scripts/project-sync-jwt.sh
var ProjectSyncJWTScript string

//go:embed scripts/project-sync-password.sh
var ProjectSyncPasswordScript string

//go:embed scripts/project-envoy-init-container.sh
var ProjectEnvoyInitContainer string

//go:embed functions/main/index.ts
var MainFunction string

//go:embed envoy/envoy.yaml.tmpl
var EnvoyBaseTemplate string

//go:embed envoy/cds.tmpl
var EnvoyCDSTemplate string

//go:embed envoy/lds.tmpl
var EnvoyLDSTemplate string
