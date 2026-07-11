#!/usr/bin/env bash
#
# Requires bash (not sh) for pipefail, which ensures failures in piped
# commands are caught during the upgrade.
#
# Upgrade Supabase Postgres from 15 to 17 on Kubernetes (Helm chart).
#
# This is the Kubernetes equivalent of the Docker upgrade script
# (supabase/docker/utils/upgrade-pg17.sh). It uses kubectl and helm
# instead of docker compose to orchestrate the pg_upgrade process.
#
# Uses Supabase's pg_upgrade scripts (initiate.sh + complete.sh) inside
# temporary Kubernetes Pods, then updates the Helm release to PG 17.
#
# Usage:
#   bash scripts/upgrade-pg17.sh -n <namespace> -r <release>           # Interactive
#   bash scripts/upgrade-pg17.sh -n <namespace> -r <release> --yes     # Non-interactive
#
# Options:
#   -n, --namespace   Kubernetes namespace where Supabase is deployed (required)
#   -r, --release     Helm release name (required)
#   -c, --chart       Path to Supabase Helm chart (default: auto-detect from release)
#   --yes, -y         Skip confirmation prompts (non-interactive mode)
#   --help            Show this help message
#
# Requirements:
#   - kubectl configured with access to the cluster
#   - helm v3
#   - Running Supabase Helm deployment with Postgres 15
#   - PVC with enough free space for pg_upgrade (~2x current data)
#
# Backup:
#   The original Postgres 15 data is preserved as a subdirectory
#   (postgres-data.bak.pg15) inside the DB PVC during the upgrade.
#   DO NOT DELETE it until you have verified the upgrade was successful.
#
# Rollback (if the upgrade fails or you want to revert):
#   1. helm upgrade <release> <chart> --set image.db.tag=15.8.1.085 \
#        --set image.initDb.tag=15-alpine --reuse-values -n <namespace>
#   2. kubectl exec -n <namespace> <db-pod> -- bash -c \
#        'rm -rf /var/lib/postgresql/data/postgres-data && \
#         mv /var/lib/postgresql/data/postgres-data.bak.pg15 \
#            /var/lib/postgresql/data/postgres-data'
#   3. kubectl rollout restart statefulset -n <namespace> <db-statefulset>
#

# Ensure we're running under bash (not sh/zsh/dash).
case "${BASH:-}" in
    */bash) ;;
    *) echo "Error: This script requires bash. Run it with: bash $0" >&2; exit 1 ;;
esac

set -euo pipefail

# --- Argument Parsing -------------------------------------------------------

AUTO_CONFIRM=false
NAMESPACE=""
RELEASE=""
CHART_PATH=""

print_usage() {
    echo "Usage: bash $0 -n <namespace> -r <release> [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -n, --namespace   Kubernetes namespace (required)"
    echo "  -r, --release     Helm release name (required)"
    echo "  -c, --chart       Path to Supabase Helm chart (default: auto-detect)"
    echo "  --yes, -y         Skip confirmation prompts"
    echo "  --help            Show this help message"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -n|--namespace) NAMESPACE="$2"; shift 2 ;;
        -r|--release) RELEASE="$2"; shift 2 ;;
        -c|--chart) CHART_PATH="$2"; shift 2 ;;
        --yes|-y) AUTO_CONFIRM=true; shift ;;
        --help) print_usage; exit 0 ;;
        *) echo "Unknown option: $1"; print_usage; exit 1 ;;
    esac
done

if [ -z "$NAMESPACE" ] || [ -z "$RELEASE" ]; then
    echo "Error: --namespace and --release are required."
    echo ""
    print_usage
    exit 1
fi

# --- Configuration ----------------------------------------------------------

# Image used for the upgrade tarball + complete.sh pod.
# Must share glibc with PG 15 (the extracted ELF binaries run inside PG15).
# Pinned to .063: later images bumped glibc, which breaks the ELF extraction.
PG17_UPGRADE_IMAGE="supabase/postgres:17.6.1.063"
# Tag in supabase/postgres repo matching the upgrade image (for downloading scripts)
PG17_SCRIPTS_REF="17.6.1.063"

# Final Postgres 17 image that runs after the upgrade. This is what the Helm
# release will be upgraded to. Keep in sync with values.yaml image.db.tag.
PG17_TARGET_IMAGE="supabase/postgres:17.6.1.136"
PG17_TARGET_TAG="17.6.1.136"
PG17_INITDB_TAG="17-alpine"

# Pod names for temporary upgrade pods
UPGRADE_POD="supabase-pg-upgrade"
COMPLETE_POD="supabase-pg-complete"
TARBALL_POD="supabase-pg-tarball"
SWAP_POD="supabase-pg-swap"

# Kubernetes label selectors (matches Helm chart conventions)
DB_LABEL_SELECTOR=""  # Set during preflight

# PVC and StatefulSet names (detected during preflight)
DB_STS_NAME=""
DB_PVC_NAME=""
PGSODIUM_PVC_NAME=""
DB_POD_NAME=""
PG_PASSWORD=""
CURRENT_IMAGE=""
CURRENT_TAG=""

# The Helm chart uses subPath: postgres-data inside the PVC.
# All data operations must work within this subpath.
DATA_SUBPATH="postgres-data"
BACKUP_SUBPATH="postgres-data.bak.pg15"
MIGRATION_SUBPATH="data_migration"

# --- Helpers ----------------------------------------------------------------

die() { printf 'Error: %s\n' "$*" >&2; exit 1; }
info() { printf '\n==> %s\n' "$*"; }
warn() { printf 'Warning: %s\n' "$*" >&2; }

confirm() {
    if [ "$AUTO_CONFIRM" = true ]; then return 0; fi
    if ! test -t 0; then
        die "This script must be run interactively, or use --yes to skip prompts."
    fi
    printf '%s (y/N) ' "$1"
    read -r reply
    case "$reply" in
        [Yy]*) return 0 ;;
        *) echo "Aborted."; exit 0 ;;
    esac
}

# Remove leftover temporary pods on exit.
cleanup() {
    kubectl delete pod "$UPGRADE_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    kubectl delete pod "$COMPLETE_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    kubectl delete pod "$TARBALL_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    kubectl delete pod "supabase-fix-pgsodium" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
}
trap cleanup EXIT

on_interrupt() {
    echo ""
    warn "Interrupted. Cleaning up temporary pods..."
    die "Interrupted."
}
trap on_interrupt INT

run_sql() {
    kubectl exec -n "$NAMESPACE" "$1" -- \
        psql -h localhost -U supabase_admin -d postgres -v ON_ERROR_STOP=1 "${@:2}"
}

# JSON-encode a string value (escape quotes, backslashes, control chars).
json_encode_string() {
    local val=$1
    # Escape backslashes first, then double quotes
    val="${val//\\/\\\\}"
    val="${val//\"/\\\"}"
    echo "\"$val\""
}

wait_for_pod_ready() {
    local pod=$1 timeout=${2:-120}
    echo "  Waiting for pod $pod to be ready (${timeout}s timeout)..."
    if ! kubectl wait -n "$NAMESPACE" "pod/$pod" --for=condition=Ready --timeout="${timeout}s" 2>/dev/null; then
        warn "Pod $pod did not become Ready in ${timeout}s."
        echo "  Pod status:"
        kubectl get pod -n "$NAMESPACE" "$pod" -o wide 2>/dev/null || true
        echo "  Pod events:"
        kubectl describe pod -n "$NAMESPACE" "$pod" 2>/dev/null | grep -A 20 "^Events:" || true
        die "Pod $pod failed to start."
    fi
}

wait_for_pg_ready() {
    local pod=$1 retries=${2:-30}
    echo "  Waiting for Postgres to accept connections..."
    while [ $retries -gt 0 ]; do
        if kubectl exec -n "$NAMESPACE" "$pod" -- pg_isready -U postgres -h localhost >/dev/null 2>&1; then
            return 0
        fi
        retries=$((retries - 1))
        sleep 2
    done
    die "Postgres in pod '$pod' did not become ready."
}

# Create a temporary pod with the DB PVC and pgsodium PVC mounted.
# Usage: create_temp_pod <pod-name> <image> [extra-env-json]
# The pod runs 'sleep infinity' and must be deleted by the caller.
create_temp_pod() {
    local pod_name=$1 image=$2 extra_env=${3:-"[]"}

    # Build volume and volumeMount specs
    local volumes="[
        {\"name\": \"db-data\", \"persistentVolumeClaim\": {\"claimName\": \"$DB_PVC_NAME\"}},
        {\"name\": \"pgsodium\", \"persistentVolumeClaim\": {\"claimName\": \"$PGSODIUM_PVC_NAME\"}}
    ]"
    local volume_mounts="[
        {\"name\": \"db-data\", \"mountPath\": \"/mnt/db-data\"},
        {\"name\": \"pgsodium\", \"mountPath\": \"/etc/postgresql-custom\"}
    ]"

    kubectl run "$pod_name" -n "$NAMESPACE" \
        --image="$image" \
        --restart=Never \
        --overrides="{
            \"apiVersion\": \"v1\",
            \"spec\": {
                \"containers\": [{
                    \"name\": \"$pod_name\",
                    \"image\": \"$image\",
                    \"command\": [\"sleep\", \"infinity\"],
                    \"env\": $extra_env,
                    \"volumeMounts\": $volume_mounts
                }],
                \"volumes\": $volumes,
                \"restartPolicy\": \"Never\"
            }
        }" >/dev/null 2>&1

    wait_for_pod_ready "$pod_name" 120
}

# --- Pre-flight checks -----------------------------------------------------

preflight() {
    info "Running pre-flight checks"

    # Check required tools
    command -v kubectl >/dev/null 2>&1 || die "kubectl not found."
    command -v helm >/dev/null 2>&1 || die "helm not found."
    command -v curl >/dev/null 2>&1 || die "curl is required (for downloading upgrade scripts)."

    # Check namespace exists
    kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 \
        || die "Namespace '$NAMESPACE' not found."

    # Check Helm release exists
    helm status "$RELEASE" -n "$NAMESPACE" >/dev/null 2>&1 \
        || die "Helm release '$RELEASE' not found in namespace '$NAMESPACE'."

    # Auto-detect chart path from release if not provided.
    # If still not found, try to derive it from the script location.
    if [ -z "$CHART_PATH" ]; then
        CHART_PATH=$(helm get metadata "$RELEASE" -n "$NAMESPACE" -o json 2>/dev/null \
            | grep -o '"chart":"[^"]*"' | cut -d'"' -f4 || true)
    fi

    if [ -z "$CHART_PATH" ]; then
        # Try same directory as this script (e.g., script lives in charts/supabase/scripts/)
        local script_dir
        script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
        if [ -f "$script_dir/../Chart.yaml" ]; then
            CHART_PATH=$(cd "$script_dir/.." && pwd)
        elif [ -f "$script_dir/Chart.yaml" ]; then
            CHART_PATH=$script_dir
        fi
    fi

    if [ -n "$CHART_PATH" ] && [ ! -f "$CHART_PATH/Chart.yaml" ]; then
        # If the path is a chart name rather than a directory, check common relative paths
        if [ ! -d "$CHART_PATH" ]; then
            warn "Chart path '$CHART_PATH' does not exist as a directory."
            local script_dir
            script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
            if [ -f "$script_dir/../Chart.yaml" ]; then
                CHART_PATH=$(cd "$script_dir/.." && pwd)
                warn "Falling back to chart directory: $CHART_PATH"
            elif [ -f "$script_dir/Chart.yaml" ]; then
                CHART_PATH=$script_dir
                warn "Falling back to chart directory: $CHART_PATH"
            else
                CHART_PATH=""
            fi
        fi
    fi

    if [ -z "$CHART_PATH" ]; then
        warn "Could not auto-detect chart path."
        warn "Please specify the chart path with: -c <path-to-chart>"
    else
        echo "  Chart path:       $CHART_PATH"
    fi

    # Find the DB StatefulSet
    DB_STS_NAME=$(kubectl get statefulset -n "$NAMESPACE" \
        -l "app.kubernetes.io/instance=$RELEASE" \
        -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null \
        | grep -E '(^|-|_)db($|-|_)' | head -n 1)

    if [ -z "$DB_STS_NAME" ]; then
        # Fallback: try common naming patterns
        DB_STS_NAME=$(kubectl get statefulset -n "$NAMESPACE" \
            -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null \
            | grep -E "(^${RELEASE}.*db|supabase.*db)" | head -n 1)
    fi
    [ -n "$DB_STS_NAME" ] || die "Could not find DB StatefulSet for release '$RELEASE'."
    echo "  StatefulSet: $DB_STS_NAME"

    # Get the DB pod name (if running)
    DB_POD_NAME=$(kubectl get pods -n "$NAMESPACE" \
        -l "statefulset.kubernetes.io/pod-name" \
        --field-selector=status.phase=Running \
        -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null \
        | grep "^${DB_STS_NAME}" | head -n 1)

    if [ -z "$DB_POD_NAME" ]; then
        DB_POD_NAME="${DB_STS_NAME}-0"
    fi
    echo "  DB Pod: $DB_POD_NAME"

    # Get current image
    CURRENT_IMAGE=$(kubectl get statefulset -n "$NAMESPACE" "$DB_STS_NAME" \
        -o jsonpath='{.spec.template.spec.containers[0].image}')
    CURRENT_TAG="${CURRENT_IMAGE##*:}"
    echo "  Current image: $CURRENT_IMAGE"

    # Find PVC names
    DB_PVC_NAME=$(kubectl get statefulset -n "$NAMESPACE" "$DB_STS_NAME" \
        -o jsonpath='{.spec.template.spec.volumes[?(@.name=="postgres-volume")].persistentVolumeClaim.claimName}' 2>/dev/null)
    [ -n "$DB_PVC_NAME" ] || die "Could not find DB PVC. Is persistence.db.enabled=true?"
    echo "  DB PVC: $DB_PVC_NAME"

    PGSODIUM_PVC_NAME=$(kubectl get statefulset -n "$NAMESPACE" "$DB_STS_NAME" \
        -o jsonpath='{.spec.template.spec.volumes[?(@.name=="pgsodium")].persistentVolumeClaim.claimName}' 2>/dev/null)
    [ -n "$PGSODIUM_PVC_NAME" ] || die "Could not find pgsodium PVC. Is persistence.pgsodium.enabled=true?"
    echo "  Pgsodium PVC: $PGSODIUM_PVC_NAME"

    # Get Postgres password from the Secret
    local secret_name
    secret_name=$(kubectl get statefulset -n "$NAMESPACE" "$DB_STS_NAME" \
        -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="PGPASSWORD")].valueFrom.secretKeyRef.name}' 2>/dev/null)
    local secret_key
    secret_key=$(kubectl get statefulset -n "$NAMESPACE" "$DB_STS_NAME" \
        -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="PGPASSWORD")].valueFrom.secretKeyRef.key}' 2>/dev/null)

    if [ -n "$secret_name" ] && [ -n "$secret_key" ]; then
        PG_PASSWORD=$(kubectl get secret -n "$NAMESPACE" "$secret_name" \
            -o jsonpath="{.data.$secret_key}" 2>/dev/null | base64 -d 2>/dev/null)
    fi
    [ -n "$PG_PASSWORD" ] || die "Could not retrieve Postgres password from Secret '$secret_name'."

    # Check StatefulSet is running
    local ready_replicas
    ready_replicas=$(kubectl get statefulset -n "$NAMESPACE" "$DB_STS_NAME" \
        -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    [ "$ready_replicas" -ge 1 ] 2>/dev/null \
        || die "DB StatefulSet '$DB_STS_NAME' has no ready replicas. Is Postgres running?"

    # Verify the pod is actually running PG 15
    local pg_version
    pg_version=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        psql -h localhost -U postgres -At -c "SHOW server_version;" 2>/dev/null | head -n 1) || true
    echo "  Postgres version: $pg_version"
    case "$pg_version" in
        15.*) ;;
        17.*) die "Already running Postgres 17." ;;
        "") warn "Could not query Postgres version." ;;
        *) warn "Unexpected Postgres version: $pg_version" ;;
    esac

    # Check for existing backup
    local has_backup
    has_backup=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        bash -c "[ -d '/var/lib/postgresql/data/$BACKUP_SUBPATH' ] && echo yes || echo no" 2>/dev/null)
    if [ "$has_backup" = "yes" ]; then
        warn "Backup directory already exists: $BACKUP_SUBPATH (inside DB PVC)"
        warn "This is likely from a previous upgrade attempt."
        warn "If you haven't verified that previous upgrade, roll back first."
        echo ""
        warn "Continuing will DELETE the existing backup permanently."
        confirm "Delete existing backup and start a fresh upgrade?"
        kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
            rm -rf "/var/lib/postgresql/data/$BACKUP_SUBPATH"
    fi

    # Clean up leftover migration dir
    kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        rm -rf "/var/lib/postgresql/data/$MIGRATION_SUBPATH" 2>/dev/null || true
}

# --- Check incompatible extensions -----------------------------------------

drop_extensions=""

check_extensions() {
    info "Checking for incompatible extensions"

    local incompatible
    incompatible=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        psql -h localhost -U supabase_admin -d postgres -At -c "
            SELECT string_agg(extname, ', ')
            FROM pg_extension
            WHERE extname IN ('timescaledb', 'plv8', 'plcoffee', 'plls');
        " 2>/dev/null | tr -d '[:space:]') || true

    if [ -n "$incompatible" ]; then
        warn "Incompatible extensions found: $incompatible"
        warn "These do not exist in Postgres 17 and must be dropped before upgrading."
        warn "If you proceed, they will be dropped automatically."
        warn "The original data is preserved as a backup so you can roll back."
        confirm "Drop these extensions and continue with the upgrade?"
        drop_extensions="$incompatible"
    fi
}

drop_incompatible_extensions() {
    if [ -z "$drop_extensions" ]; then
        return
    fi
    info "Dropping incompatible extensions"

    local ext
    echo "$drop_extensions" | tr ',' '\n' | while read -r ext; do
        ext=$(echo "$ext" | tr -d '[:space:]')
        [ -z "$ext" ] && continue
        echo "  DROP EXTENSION $ext CASCADE"
        kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
            psql -h localhost -U supabase_admin -d postgres \
            -c "DROP EXTENSION IF EXISTS \"$ext\" CASCADE;"
    done
}

# --- Show summary and confirm -----------------------------------------------

show_summary() {
    echo ""
    echo "This script will:"
    echo "  1. Build an upgrade tarball from the PG 17 image"
    echo "  2. Scale down all Supabase services"
    echo "  3. Run pg_upgrade (Postgres 15 -> 17) in a temporary Pod"
    echo "  4. Apply post-upgrade patches in a PG 17 Pod"
    echo "  5. Swap data directories inside the PVC"
    echo "  6. Helm upgrade to Postgres 17 image"
    echo "  7. Apply additional migrations"
    echo ""
    echo "  Namespace:        $NAMESPACE"
    echo "  Helm release:     $RELEASE"
    echo "  StatefulSet:      $DB_STS_NAME"
    echo "  Current image:    $CURRENT_IMAGE"
    echo "  Upgrade image:    $PG17_UPGRADE_IMAGE"
    echo "  Target image:     $PG17_TARGET_IMAGE"
    echo "  DB PVC:           $DB_PVC_NAME"
    echo "  Pgsodium PVC:     $PGSODIUM_PVC_NAME"
    echo "  Data subpath:     $DATA_SUBPATH"
    echo "  Backup subpath:   $BACKUP_SUBPATH"
    echo ""
    confirm "Proceed with the upgrade?"
}

# --- Step 1: Build upgrade tarball ------------------------------------------
#
# Creates a temporary Pod that extracts PG 17 binaries from the upgrade image
# and stores the tarball inside the DB PVC (so it's available to subsequent pods).
# This is the Kubernetes equivalent of Docker's build_tarball().

build_tarball() {
    info "Building upgrade tarball from Postgres 17 image"

    # Check if tarball is already cached in the PVC
    local has_tarball
    has_tarball=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        bash -c "[ -f '/var/lib/postgresql/data/pg17_upgrade_bin.tar.gz' ] && echo yes || echo no" 2>/dev/null)

    if [ "$has_tarball" = "yes" ]; then
        info "Using cached upgrade tarball from PVC"
        return
    fi

    echo "  Starting tarball builder pod..."

    # Create a long-running pod with the PG17 image + DB PVC mounted.
    # We use a temporary directory inside the PVC itself (/mnt/db-data/_pg17_export)
    # for staging the extracted binaries, avoiding emptyDir size limits that
    # can cause OOM kills on resource-constrained clusters.
    kubectl run "$TARBALL_POD" -n "$NAMESPACE" \
        --image="$PG17_UPGRADE_IMAGE" \
        --restart=Never \
        --overrides="{
            \"apiVersion\": \"v1\",
            \"spec\": {
                \"containers\": [{
                    \"name\": \"$TARBALL_POD\",
                    \"image\": \"$PG17_UPGRADE_IMAGE\",
                    \"command\": [\"sleep\", \"infinity\"],
                    \"volumeMounts\": [
                        {\"name\": \"db-data\", \"mountPath\": \"/mnt/db-data\"}
                    ]
                }],
                \"volumes\": [
                    {\"name\": \"db-data\", \"persistentVolumeClaim\": {\"claimName\": \"$DB_PVC_NAME\"}}
                ],
                \"restartPolicy\": \"Never\"
            }
        }" >/dev/null 2>&1

    wait_for_pod_ready "$TARBALL_POD" 120

    info "Extracting PG 17 binaries and creating tarball (this may take several minutes)"

    # Run the build script inside the pod via kubectl exec.
    # Staging directory is inside the PVC to avoid emptyDir memory limits.
    if ! kubectl exec -n "$NAMESPACE" "$TARBALL_POD" -- bash -c '
        set -euo pipefail
        EXPORT=/mnt/db-data/_pg17_export

        rm -rf "$EXPORT"
        mkdir -p "$EXPORT/17/bin" "$EXPORT/17/lib" "$EXPORT/17/share"

        echo "  Copying binaries..."
        BIN_DIR=$(dirname $(readlink -f /usr/lib/postgresql/bin/postgres))
        for f in "$BIN_DIR"/*; do
            name=$(basename "$f")
            case "$name" in .*-wrapped) continue ;; esac
            if [ -x "$f" ] && file -b "$f" | grep -q "ELF .* executable"; then
                cp "$f" "$EXPORT/17/bin/$name"
            else
                wrapped=$(grep -o "/nix/store/[^ \"]*-wrapped" "$f" 2>/dev/null | head -n 1 || true)
                if [ -n "$wrapped" ] && [ -f "$wrapped" ]; then
                    cp "$wrapped" "$EXPORT/17/bin/$name"
                else
                    cp "$f" "$EXPORT/17/bin/$name"
                fi
            fi
        done

        echo "  Copying libraries..."
        PKGLIBDIR=$(pg_config --pkglibdir)
        LIBDIR=$(pg_config --libdir)
        cp -Lf "$PKGLIBDIR"/*.so "$EXPORT/17/lib/" 2>/dev/null || true
        cp -Lf "$LIBDIR"/*.so* "$EXPORT/17/lib/" 2>/dev/null || true
        cp -Lf /nix/var/nix/profiles/default/lib/*.so* "$EXPORT/17/lib/" 2>/dev/null || true

        echo "  Copying share data..."
        mkdir -p "$EXPORT/17/share/postgresql"
        rm -f /usr/share/postgresql/timezonesets/timezonesets 2>/dev/null || true
        mkdir -p "$EXPORT/17/share/postgresql"/{extension,timezonesets,tsearch_data}
        mkdir -p "$EXPORT/17/share/postgresql"/extension/{functions,procedures,tables,types}
        cp -rL /usr/share/postgresql/* "$EXPORT/17/share/postgresql/" 2>/dev/null || true

        echo "  Copying extension definitions to lib..."
        SHAREDIR=$(pg_config --sharedir)
        cp "$SHAREDIR"/extension/*.control "$EXPORT/17/lib/" 2>/dev/null || true
        cp "$SHAREDIR"/extension/*.sql "$EXPORT/17/lib/" 2>/dev/null || true

        echo "  Checking key files..."
        [ -f "$EXPORT/17/bin/postgres" ] || { echo "Error: bin/postgres missing"; exit 1; }
        [ -f "$EXPORT/17/share/postgresql/timezonesets/Default" ] || { echo "Error: timezonesets/Default missing"; exit 1; }
        ls "$EXPORT/17/share/postgresql/extension"/*.control >/dev/null 2>&1 || { echo "Error: no .control files"; exit 1; }
        ls "$EXPORT/17/lib"/*.so >/dev/null 2>&1 || { echo "Error: no .so files"; exit 1; }

        echo "  Creating tarball (this may take several minutes)..."
        cd "$EXPORT" && tar czf /mnt/db-data/pg17_upgrade_bin.tar.gz 17/
        echo "  Tarball created: $(du -sh /mnt/db-data/pg17_upgrade_bin.tar.gz | cut -f1)"

        echo "  Cleaning up staging directory..."
        rm -rf "$EXPORT"
    '; then
        echo ""
        echo "  Tarball pod logs:"
        kubectl logs -n "$NAMESPACE" "$TARBALL_POD" 2>/dev/null || true
        kubectl delete pod "$TARBALL_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
        die "Tarball build failed."
    fi

    info "Tarball built successfully"
    kubectl delete pod "$TARBALL_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
}

# --- Step 2: Download upgrade scripts --------------------------------------

download_scripts_into_pod() {
    local pod=$1
    local scripts_base="https://raw.githubusercontent.com/supabase/postgres/${PG17_SCRIPTS_REF}/ansible/files/admin_api_scripts/pg_upgrade_scripts"

    info "Downloading upgrade scripts into pod $pod"
    kubectl exec -n "$NAMESPACE" "$pod" -- mkdir -p /tmp/upgrade /tmp/persistent /tmp/pg_upgrade

    for script in initiate.sh complete.sh common.sh pgsodium_getkey.sh check.sh prepare.sh; do
        echo "  Downloading $script..."
        local content
        content=$(curl -fsSL "$scripts_base/$script") \
            || die "Failed to download $script from GitHub"
        # Write script into the pod
        echo "$content" | kubectl exec -n "$NAMESPACE" "$pod" -i -- bash -c "cat > /tmp/upgrade/$script"
        kubectl exec -n "$NAMESPACE" "$pod" -- chmod +x "/tmp/upgrade/$script"
    done
}

# --- Step 3: Backup pgsodium key -------------------------------------------

backup_pgsodium() {
    info "Backing up pgsodium root key"
    # Copy the key to a backup location inside the DB PVC
    kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- bash -c \
        "cp /etc/postgresql-custom/pgsodium_root.key /var/lib/postgresql/data/pgsodium_root.key.bak.pg15 2>/dev/null || echo 'No pgsodium key found (may be OK for fresh installs)'"
    echo "  Key backed up inside DB PVC"
}

# --- Step 4: Scale down all services ----------------------------------------

# Track original replica counts for potential rollback info
declare -A ORIGINAL_REPLICAS 2>/dev/null || true

scale_down() {
    info "Scaling down all Supabase services"

    # Get all deployments and statefulsets belonging to this release
    local deployments statefulsets

    deployments=$(kubectl get deployment -n "$NAMESPACE" \
        -l "app.kubernetes.io/instance=$RELEASE" \
        -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.replicas}{"\n"}{end}' 2>/dev/null) || true

    statefulsets=$(kubectl get statefulset -n "$NAMESPACE" \
        -l "app.kubernetes.io/instance=$RELEASE" \
        -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.replicas}{"\n"}{end}' 2>/dev/null) || true

    # Scale down deployments first (non-DB services)
    while IFS=' ' read -r name replicas; do
        [ -z "$name" ] && continue
        echo "  Scaling down deployment/$name (was: ${replicas:-1} replicas)"
        kubectl scale deployment -n "$NAMESPACE" "$name" --replicas=0 >/dev/null 2>&1 || true
    done <<< "$deployments"

    # Wait for deployments to scale down
    sleep 5

    # Scale down statefulsets (includes DB)
    while IFS=' ' read -r name replicas; do
        [ -z "$name" ] && continue
        echo "  Scaling down statefulset/$name (was: ${replicas:-1} replicas)"
        kubectl scale statefulset -n "$NAMESPACE" "$name" --replicas=0 >/dev/null 2>&1 || true
    done <<< "$statefulsets"

    # Wait for DB pod to terminate
    echo "  Waiting for DB pod to terminate..."
    local retries=60
    while [ $retries -gt 0 ]; do
        if ! kubectl get pod -n "$NAMESPACE" "$DB_POD_NAME" >/dev/null 2>&1; then
            break
        fi
        retries=$((retries - 1))
        sleep 2
    done
    [ $retries -gt 0 ] || die "DB pod did not terminate within 120 seconds."
    echo "  All services scaled down."
}

# --- Step 5: Run pg_upgrade via initiate.sh ---------------------------------
#
# Creates a temporary pod using the current PG 15 image with the DB PVC
# mounted. The tarball (already in the PVC) provides PG 17 binaries.
# initiate.sh runs pg_upgrade to create the upgraded data directory.

run_upgrade() {
    info "Starting upgrade pod (PG 15 + PG 17 binaries)"

    # Include all env vars needed by initiate.sh in the pod spec so they're
    # available to every kubectl exec invocation without --env (unsupported).
    local pw_json
    pw_json=$(json_encode_string "$PG_PASSWORD")
    local env_json="[
        {\"name\": \"PGPASSWORD\", \"value\": $pw_json},
        {\"name\": \"IS_CI\", \"value\": \"true\"},
        {\"name\": \"PG_MAJOR_VERSION\", \"value\": \"17\"},
        {\"name\": \"LD_LIBRARY_PATH\", \"value\": \"/tmp/pg_upgrade_bin/17/lib\"},
        {\"name\": \"NIX_PGLIBDIR\", \"value\": \"/tmp/pg_upgrade_bin/17/lib\"}
    ]"

    create_temp_pod "$UPGRADE_POD" "$CURRENT_IMAGE" "$env_json"

    info "Preparing upgrade environment"
    kubectl exec -n "$NAMESPACE" "$UPGRADE_POD" -- bash -c "
        # Create symlinks: the upgrade scripts expect data at /var/lib/postgresql/data
        # but our PVC is mounted at /mnt/db-data with subpath $DATA_SUBPATH
        rm -rf /var/lib/postgresql/data
        ln -s /mnt/db-data/$DATA_SUBPATH /var/lib/postgresql/data

        # Create migration directory inside PVC
        mkdir -p /mnt/db-data/$MIGRATION_SUBPATH
        ln -s /mnt/db-data/$MIGRATION_SUBPATH /data_migration

        # Copy tarball from PVC to where initiate.sh expects it.
        # initiate.sh may run as 'postgres' user, so make it world-readable.
        mkdir -p /tmp/persistent /tmp/upgrade /tmp/pg_upgrade
        chmod 755 /tmp/persistent
        echo '  Copying tarball from PVC to /tmp/persistent...'
        ls -lh /mnt/db-data/pg17_upgrade_bin.tar.gz
        cp /mnt/db-data/pg17_upgrade_bin.tar.gz /tmp/persistent/pg_upgrade_bin.tar.gz
        chmod 644 /tmp/persistent/pg_upgrade_bin.tar.gz
        chown postgres:postgres /tmp/persistent/pg_upgrade_bin.tar.gz
        echo '  /tmp/persistent contents:'
        ls -lh /tmp/persistent/
    "

    # Download scripts and apply patches
    download_scripts_into_pod "$UPGRADE_POD"

    kubectl exec -n "$NAMESPACE" "$UPGRADE_POD" -- bash -c '
        # Patch CI_start_postgres to use "restart" instead of "start" for idempotency
        sed -i "s/pg_ctl start -o/pg_ctl restart -o/g" /tmp/upgrade/common.sh

        # Patch PGSHARENEW to match nix binary expectations (share/postgresql/)
        sed -i "s|PGSHARENEW=\"\$PG_UPGRADE_BIN_DIR/share\"|PGSHARENEW=\"\$PG_UPGRADE_BIN_DIR/share/postgresql\"|" /tmp/upgrade/initiate.sh
    '

    info "Starting Postgres 15 in upgrade pod"
    kubectl exec -n "$NAMESPACE" "$UPGRADE_POD" -- bash -c \
        'su postgres -c "pg_ctl start -o \"-c config_file=/etc/postgresql/postgresql.conf\" -l /tmp/postgres.log"'
    wait_for_pg_ready "$UPGRADE_POD" 30

    info "Running initiate.sh (pg_upgrade: Postgres 15 -> 17)"
    echo "  This may take several minutes depending on database size..."
    echo ""
    # Env vars (IS_CI, PG_MAJOR_VERSION, PGPASSWORD, LD_LIBRARY_PATH,
    # NIX_PGLIBDIR) are already set in the pod spec via create_temp_pod.
    if ! kubectl exec -n "$NAMESPACE" "$UPGRADE_POD" -- \
        /tmp/upgrade/initiate.sh 17 2>&1; then
        echo ""
        warn "initiate.sh failed. Your data directory is unchanged."
        warn "Check the output above for the root cause, fix it, and re-run."
        kubectl delete pod "$UPGRADE_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
        die "initiate.sh failed"
    fi

    info "initiate.sh completed successfully"
    kubectl delete pod "$UPGRADE_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true
}

# --- Step 6: Run complete.sh in a PG 17 pod ---------------------------------
#
# complete.sh applies post-upgrade patches (pg_net grants, vault re-encryption,
# pg_cron, predefined roles, vacuumdb, etc.).

run_complete() {
    info "Starting PG 17 pod for complete.sh"

    # Include all env vars needed by complete.sh in the pod spec.
    local pw_json
    pw_json=$(json_encode_string "$PG_PASSWORD")
    local env_json="[
        {\"name\": \"PGPASSWORD\", \"value\": $pw_json},
        {\"name\": \"IS_CI\", \"value\": \"true\"},
        {\"name\": \"PG_MAJOR_VERSION\", \"value\": \"17\"}
    ]"

    create_temp_pod "$COMPLETE_POD" "$PG17_UPGRADE_IMAGE" "$env_json"

    info "Preparing complete.sh environment"

    # Save original pgsodium ownership for potential rollback
    kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- bash -c '
        stat -c "%u:%g" /etc/postgresql-custom/pgsodium_root.key 2>/dev/null > /tmp/dbconfig_owner || true
    '

    kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- bash -c "
        # Symlink migration dir
        ln -s /mnt/db-data/$MIGRATION_SUBPATH /data_migration

        # Remove default data dir (complete.sh creates a symlink here)
        rm -rf /var/lib/postgresql/data

        # Fix ownership on pgsodium volume (PG15 uid differs from PG17)
        chown -R postgres:postgres /etc/postgresql-custom/

        # PG17 config includes this directory; may not exist from PG15
        mkdir -p /etc/postgresql-custom/conf.d
    "

    # Download scripts and apply patches
    download_scripts_into_pod "$COMPLETE_POD"

    kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- bash -c '
        # Patch --new-bin to use native bindir (we are in a PG17 container)
        sed -i "s|BINDIR=\"/tmp/pg_upgrade_bin/\$PG_MAJOR_VERSION/bin\"|BINDIR=\$(pg_config --bindir)|g" /tmp/upgrade/common.sh
    '

    info "Running complete.sh (post-upgrade patches, vacuum analyze)"
    # Env vars (IS_CI, PG_MAJOR_VERSION, PGPASSWORD) are already set in the
    # pod spec via create_temp_pod.
    kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- \
        /tmp/upgrade/complete.sh 2>&1 || true

    # Check status file
    local status
    status=$(kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- \
        cat /tmp/pg-upgrade-status 2>/dev/null || echo "unknown")
    if [ "$status" != "complete" ]; then
        warn "complete.sh failed. Postgres log:"
        kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- \
            cat /tmp/postgres.log 2>/dev/null || true
        echo ""

        # Restore pgsodium ownership for PG15 rollback
        warn "Restoring pgsodium ownership for PG15..."
        local orig_owner
        orig_owner=$(kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- \
            cat /tmp/dbconfig_owner 2>/dev/null || true)
        if [ -n "$orig_owner" ]; then
            kubectl exec -n "$NAMESPACE" "$COMPLETE_POD" -- \
                chown -R "$orig_owner" /etc/postgresql-custom/ 2>/dev/null || true
        fi

        kubectl delete pod "$COMPLETE_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
        echo ""
        echo "  Your Postgres 15 data is unchanged (data swap has not happened yet)."
        echo "  To restart Postgres 15, scale up the DB StatefulSet:"
        echo "    kubectl scale statefulset -n $NAMESPACE $DB_STS_NAME --replicas=1"
        echo ""
        die "complete.sh failed (status: $status)"
    fi

    info "complete.sh finished successfully"
    kubectl delete pod "$COMPLETE_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true
}

# --- Step 7: Swap data directories inside PVC --------------------------------
#
# Uses a lightweight alpine pod to rename directories inside the PVC.
# The Helm chart mounts with subPath: postgres-data, so we:
#   - Rename postgres-data -> postgres-data.bak.pg15
#   - Rename data_migration/pgdata -> postgres-data
#   - Remove data_migration

swap_data() {
    info "Swapping data directories inside PVC"

    # Use a lightweight pod with the DB PVC to rename directories.
    kubectl run "$SWAP_POD" -n "$NAMESPACE" \
        --image="alpine:3.20" \
        --restart=Never \
        --overrides="{
            \"apiVersion\": \"v1\",
            \"spec\": {
                \"containers\": [{
                    \"name\": \"$SWAP_POD\",
                    \"image\": \"alpine:3.20\",
                    \"command\": [\"sleep\", \"infinity\"],
                    \"volumeMounts\": [{\"name\": \"db-data\", \"mountPath\": \"/mnt/db-data\"}]
                }],
                \"volumes\": [{\"name\": \"db-data\", \"persistentVolumeClaim\": {\"claimName\": \"$DB_PVC_NAME\"}}],
                \"restartPolicy\": \"Never\"
            }
        }" >/dev/null 2>&1

    wait_for_pod_ready "$SWAP_POD" 60

    echo "  PVC contents before swap:"
    kubectl exec -n "$NAMESPACE" "$SWAP_POD" -- ls -la /mnt/db-data/ 2>/dev/null || true

    if ! kubectl exec -n "$NAMESPACE" "$SWAP_POD" -- sh -c "
        echo 'Renaming $DATA_SUBPATH -> $BACKUP_SUBPATH'
        mv /mnt/db-data/$DATA_SUBPATH /mnt/db-data/$BACKUP_SUBPATH

        echo 'Moving migration data -> $DATA_SUBPATH'
        mv /mnt/db-data/$MIGRATION_SUBPATH/pgdata /mnt/db-data/$DATA_SUBPATH

        echo 'Cleaning up migration dir'
        rm -rf /mnt/db-data/$MIGRATION_SUBPATH

        echo 'Data swap complete'
        ls -la /mnt/db-data/
    "; then
        kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
        die "Data swap failed."
    fi

    kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
}

# --- Step 8: Fix pgsodium ownership for PG 17 --------------------------------

fix_pgsodium_ownership() {
    info "Fixing pgsodium volume ownership for PG 17"

    # The PG 17 image may use a different UID for the postgres user.
    # Run a pod with the target image to chown the pgsodium volume.
    local fix_pod="supabase-fix-pgsodium"

    kubectl run "$fix_pod" -n "$NAMESPACE" \
        --image="$PG17_TARGET_IMAGE" \
        --restart=Never \
        --overrides="{
            \"apiVersion\": \"v1\",
            \"spec\": {
                \"containers\": [{
                    \"name\": \"fix-pgsodium\",
                    \"image\": \"$PG17_TARGET_IMAGE\",
                    \"command\": [\"sleep\", \"infinity\"],
                    \"volumeMounts\": [{\"name\": \"pgsodium\", \"mountPath\": \"/vol\"}]
                }],
                \"volumes\": [{\"name\": \"pgsodium\", \"persistentVolumeClaim\": {\"claimName\": \"$PGSODIUM_PVC_NAME\"}}],
                \"restartPolicy\": \"Never\"
            }
        }" >/dev/null 2>&1

    wait_for_pod_ready "$fix_pod" 120

    kubectl exec -n "$NAMESPACE" "$fix_pod" -- sh -c \
        'mkdir -p /vol/conf.d && chown -R postgres:postgres /vol/ && echo "Ownership fixed"' \
        || warn "pgsodium ownership fix failed (non-fatal)"

    kubectl delete pod "$fix_pod" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
}

# --- Step 9: Helm upgrade to PG 17 ------------------------------------------

helm_upgrade() {
    info "Upgrading Helm release to Postgres 17"

    local helm_args=(
        upgrade "$RELEASE"
        --namespace "$NAMESPACE"
        --set "image.db.tag=$PG17_TARGET_TAG"
        --set "image.initDb.tag=$PG17_INITDB_TAG"
        --reuse-values
    )

    if [ -n "$CHART_PATH" ] && [ -f "$CHART_PATH/Chart.yaml" ]; then
        helm_args+=("$CHART_PATH")
    elif [ -n "$CHART_PATH" ]; then
        warn "Chart path '$CHART_PATH' does not contain Chart.yaml."
        warn "Trying to locate chart from release metadata..."
        local chart_info
        chart_info=$(helm get metadata "$RELEASE" -n "$NAMESPACE" -o json 2>/dev/null \
            | grep -o '"chart":"[^"]*"' | cut -d'"' -f4 || true)
        if [ -n "$chart_info" ]; then
            helm_args+=("$chart_info")
        else
            echo ""
            echo "  Could not determine chart path automatically."
            echo "  Please run the helm upgrade manually:"
            echo ""
            echo "    helm upgrade $RELEASE <chart-path> \\"
            echo "      --namespace $NAMESPACE \\"
            echo "      --set image.db.tag=$PG17_TARGET_TAG \\"
            echo "      --set image.initDb.tag=$PG17_INITDB_TAG \\"
            echo "      --reuse-values"
            echo ""
            confirm "Skip automatic helm upgrade and continue with manual instructions?"
            return
        fi
    else
        echo ""
        echo "  Could not determine chart path automatically."
        echo "  Please run the helm upgrade manually:"
        echo ""
        echo "    helm upgrade $RELEASE <chart-path> \\"
        echo "      --namespace $NAMESPACE \\"
        echo "      --set image.db.tag=$PG17_TARGET_TAG \\"
        echo "      --set image.initDb.tag=$PG17_INITDB_TAG \\"
        echo "      --reuse-values"
        echo ""
        confirm "Skip automatic helm upgrade and continue with manual instructions?"
        return
    fi

    echo "  Running: helm ${helm_args[*]}"
    helm "${helm_args[@]}"

    echo "  Waiting for DB pod to be ready..."
    local retries=60
    while [ $retries -gt 0 ]; do
        DB_POD_NAME="${DB_STS_NAME}-0"
        if kubectl get pod -n "$NAMESPACE" "$DB_POD_NAME" >/dev/null 2>&1; then
            local phase
            phase=$(kubectl get pod -n "$NAMESPACE" "$DB_POD_NAME" \
                -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
            if [ "$phase" = "Running" ]; then
                break
            fi
        fi
        retries=$((retries - 1))
        sleep 3
    done
    [ $retries -gt 0 ] || die "DB pod did not start within 180 seconds after helm upgrade."

    wait_for_pg_ready "$DB_POD_NAME" 60

    # Verify PG version
    local new_version
    new_version=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        psql -h localhost -U supabase_admin -d postgres -At \
        -c "SHOW server_version;" 2>/dev/null | head -n 1)
    echo "  Postgres version: $new_version"
    case "$new_version" in
        17.*) ;;
        *) die "Expected Postgres 17.x, got: $new_version" ;;
    esac
}

# --- Step 10: Apply migrations not covered by complete.sh -------------------
#
# These PG 17 migrations run on fresh installs via initdb but not after
# pg_upgrade. complete.sh doesn't cover them either.
#
# Source: postgres/migrations/db/migrations/ (same as Docker script)

apply_role_migrations() {
    info "Applying Postgres 17 migrations"

    # Fix collation version mismatch first
    for db in postgres template1 _supabase; do
        kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
            psql -h localhost -U supabase_admin -d "$db" \
            -c "ALTER DATABASE \"$db\" REFRESH COLLATION VERSION;" 2>/dev/null || true
    done

    # Create supabase_etl_admin role
    run_sql "$DB_POD_NAME" -c "
        DO \$\$
        BEGIN
            IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_etl_admin') THEN
                CREATE USER supabase_etl_admin WITH LOGIN REPLICATION;
                GRANT pg_read_all_data TO supabase_etl_admin;
                GRANT CREATE ON DATABASE postgres TO supabase_etl_admin;
            END IF;
        END
        \$\$;" || true

    # Run migration files from the PG 17 container image
    local migration_dir="/docker-entrypoint-initdb.d/migrations"
    local migrations="
        20250710151649_supabase_read_only_user_default_transaction_read_only.sql
        20251001204436_predefined_role_grants.sql
        20251105172723_grant_pg_reload_conf_to_postgres.sql
        20251121132723_correct_search_path_pgbouncer.sql
        20260211120934_supabase_privileged_role.sql
        20260413000000_fix-authenticator-session-preload-libraries.sql
        20260421000001_rescope_pg_graphql_access_trigger.sql
    "

    for m in $migrations; do
        echo "  Running: $m"
        kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
            psql -h localhost -U supabase_admin -d postgres -v ON_ERROR_STOP=1 \
            -f "${migration_dir}/${m}" 2>/dev/null || warn "  $m failed (non-fatal)"
    done

    # Reconcile extension versions
    info "Reconciling extension versions to the target image"
    run_sql "$DB_POD_NAME" -c "
        DO \$\$
        DECLARE r record;
        BEGIN
            FOR r IN SELECT extname FROM pg_extension LOOP
                BEGIN
                    EXECUTE format('ALTER EXTENSION %I UPDATE', r.extname);
                EXCEPTION WHEN OTHERS THEN
                    RAISE NOTICE 'skipped extension %: %', r.extname, SQLERRM;
                END;
            END LOOP;
        END
        \$\$;" || warn "extension version reconcile had errors (non-fatal)"

    # pg_cron version reconciliation
    run_sql "$DB_POD_NAME" -c "
        DO \$\$
        DECLARE want text;
        BEGIN
            SELECT default_version INTO want FROM pg_available_extensions WHERE name = 'pg_cron';
            IF want IS NOT NULL
               AND EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron' AND extversion <> want) THEN
                UPDATE pg_extension SET extversion = want WHERE extname = 'pg_cron';
            END IF;
        END
        \$\$;" || warn "pg_cron version reconcile had errors (non-fatal)"
}

# --- Step 11: Verify --------------------------------------------------------

verify() {
    info "Verification"

    local version
    version=$(run_sql "$DB_POD_NAME" -At -c "SELECT version();" 2>/dev/null | head -n 1)
    echo "  $version"

    echo ""
    echo "  Extensions:"
    run_sql "$DB_POD_NAME" -c \
        "SELECT extname, extversion FROM pg_extension ORDER BY extname;"

    echo ""
    info "Upgrade complete!"
    echo ""
    echo "  Postgres is now running version 17 in namespace '$NAMESPACE'."
    echo ""
    echo "  Postgres 15 backup is preserved inside the DB PVC as: $BACKUP_SUBPATH"
    echo "  pgsodium key backup is at: pgsodium_root.key.bak.pg15 (inside DB PVC)"
    echo ""
    echo "  Once satisfied, you can reclaim space by exec'ing into the DB pod:"
    echo "    kubectl exec -n $NAMESPACE ${DB_STS_NAME}-0 -- rm -rf /var/lib/postgresql/data/$BACKUP_SUBPATH"
    echo "    kubectl exec -n $NAMESPACE ${DB_STS_NAME}-0 -- rm -f /var/lib/postgresql/data/pg17_upgrade_bin.tar.gz"
    echo ""
    echo "  Rollback (if needed):"
    echo "    1. helm upgrade $RELEASE <chart-path> \\"
    echo "         --set image.db.tag=15.8.1.085 \\"
    echo "         --set image.initDb.tag=15-alpine \\"
    echo "         --reuse-values -n $NAMESPACE"
    echo "    2. Wait for the release to apply, then scale down the DB:"
    echo "       kubectl scale statefulset -n $NAMESPACE $DB_STS_NAME --replicas=0"
    echo "    3. Wait for the DB pod to terminate, then swap data back:"
    echo "       kubectl run supabase-rollback -n $NAMESPACE --image=alpine:3.20 \\"
    echo "         --restart=Never --overrides='{\"apiVersion\":\"v1\",\"spec\":{\"containers\":[{\"name\":\"rollback\",\"image\":\"alpine:3.20\",\"command\":[\"sh\",\"-c\"],\"args\":[\"rm -rf /mnt/db-data/$DATA_SUBPATH && mv /mnt/db-data/$BACKUP_SUBPATH /mnt/db-data/$DATA_SUBPATH\"],\"volumeMounts\":[{\"name\":\"db-data\",\"mountPath\":\"/mnt/db-data\"}]}],\"volumes\":[{\"name\":\"db-data\",\"persistentVolumeClaim\":{\"claimName\":\"'\"$DB_PVC_NAME\"'\"}}],\"restartPolicy\":\"Never\"}}'"
    echo "    4. Fix pgsodium ownership for PG 15:"
    echo "       kubectl run supabase-fix-owner -n $NAMESPACE --image=$CURRENT_IMAGE \\"
    echo "         --restart=Never --overrides='{...}' -- chown -R postgres:postgres /etc/postgresql-custom/"
    echo "    5. Scale up:"
    echo "       kubectl scale statefulset -n $NAMESPACE $DB_STS_NAME --replicas=1"
    echo ""
}

# --- Main -------------------------------------------------------------------

main() {
    echo ""
    echo "Supabase Kubernetes: Postgres 15 -> 17 Upgrade"
    echo "==============================================="

    preflight
    check_extensions
    show_summary
    build_tarball
    drop_incompatible_extensions
    backup_pgsodium
    scale_down
    run_upgrade
    run_complete
    swap_data
    fix_pgsodium_ownership
    helm_upgrade
    apply_role_migrations
    verify
}

main "$@"
