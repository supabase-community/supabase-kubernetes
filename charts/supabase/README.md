# Supabase Helm chart — Deprecated

> This chart is deprecated. Use the **Supabase Kubernetes Operator** for new deployments and plan to migrate existing installations.
>
> Start with the [main README and Operator Quick Start](https://github.com/supabase-community/supabase-kubernetes#quick-start).

## Maintenance policy

The `supabase` chart receives **critical fixes only**. New features and routine image updates are no longer maintained, and automatic image-update pull requests have been disabled. The source and previously published packages remain available. No removal date has been set.

The chart keeps its own version sequence, starting with deprecation release `0.8.1`. Its version does not indicate compatibility with a Supabase self-hosted release. Future critical-fix releases will remain marked as deprecated.

Active development focuses on the Operator and the `supabase-operator` and `supabase-project` charts.

## Why move to the Operator?

The Operator manages the lifecycle of a Supabase project through Kubernetes custom resources. The following comparison describes the implementations in this repository.

| Capability | Deprecated `supabase` chart | Supabase Kubernetes Operator |
|---|---|---|
| Continuous reconciliation | Applies rendered resources during Helm installation and upgrade; has no Supabase controller watching them | Watches managed resources and reconciles them against their desired state |
| Credentials and API keys | Requires the user to supply credential values or existing Secrets, including JWT/JWKS and publishable/secret API keys | Generates database and Studio credentials, JWT/JWKS, and publishable/secret API keys for each project |
| Database credential synchronization | Has no controller to synchronize credential changes with database roles and JWT configuration | Runs and tracks password and JWT synchronization Jobs |
| SQL migrations | Supplies database initialization scripts and user SQL through the database startup configuration | Manages SQL through `Migration` resources and Jobs, and waits for migrations before completing project setup |
| Operational status | Exposes Helm release and individual workload status | Reports readiness conditions and failure reasons on Supabase custom resources |
| API gateway | Uses Kong | Manages Envoy and its configuration |
| Log collection | Deploys Vector as a Deployment | Manages Vector as a DaemonSet with per-project Kubernetes log discovery |

The chart already supports user-provided JWT/JWKS and newer API keys. The Operator adds generation and synchronization of this configuration. Both approaches still rely on Kubernetes workload controllers to manage Pods.

The Operator is in an early stage of development and its API may change. Review its supported configuration before migrating; this comparison is not a claim that every chart option has a direct equivalent.

## Plan your migration

Installing `supabase-operator` and `supabase-project` does **not** automatically convert an existing Helm installation or adopt its resources. Treat migration as a separate operation:

1. Inventory your Helm values, component versions, database, Storage backend, Edge Functions, custom configuration, Secrets, and persistent volumes.
2. Back up the database and Storage objects, and verify how to restore them. Preserve the existing installation until the replacement is validated.
3. Plan how to transfer data and map configuration, credentials, JWT signing keys, API keys, and volumes. Newly generated Operator credentials are not replacements for existing client credentials without a planned transition.
4. Deploy and validate the Operator-managed project in isolation before switching application traffic and public URLs. Check authentication, database access, Storage, Realtime, and functions used by your applications.

Follow the [Operator Quick Start](https://github.com/supabase-community/supabase-kubernetes#quick-start) for installation. It provisions a project; it is not an in-place migration procedure. Review the [upstream self-hosted changelog](https://github.com/supabase/supabase/blob/master/docker/CHANGELOG.md) and Operator release notes when changing component versions, particularly for database migrations that cannot be rolled back.

For help with this Kubernetes migration, open an issue in [this repository](https://github.com/supabase-community/supabase-kubernetes/issues).
