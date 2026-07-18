#!/usr/bin/env bash
#
# Requires bash (not sh) for pipefail, which ensures failures in piped
# commands are caught during the rollback.
#
# Roll back Supabase Postgres from 17 to 15 on Kubernetes (Helm chart).
#
# This script reverses the in-place upgrade performed by
# scripts/upgrade-pg17.sh. It restores the Postgres 15 backup directory
# inside the DB PVC, fixes pgsodium volume ownership for the PG 15 image,
# and rolls the Helm release back to the PG 15 image tags.
#
# Usage:
#   bash scripts/rollback-pg15.sh -n <namespace> -r <release>           # Interactive
#   bash scripts/rollback-pg15.sh -n <namespace> -r <release> --yes     # Non-interactive
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
#   - A Supabase Helm deployment that was previously upgraded to Postgres 17
#     by scripts/upgrade-pg17.sh
#   - The original Postgres 15 backup directory (postgres-data.bak.pg15)
#     must exist inside the DB PVC
#
# Safety:
#   This script is DESTRUCTIVE: it removes the Postgres 17 data directory
#   (postgres-data) and restores the Postgres 15 backup directory
#   (postgres-data.bak.pg15) in its place.
#
#   ALWAYS take a snapshot or backup of the DB PVC before running this script.
#   If the snapshot/backup is missing, you will not be able to recover the
#   Postgres 17 data after rollback.
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

# Target Postgres 15 image tags.
PG15_TARGET_IMAGE="supabase/postgres:15.8.1.085"
PG15_TARGET_TAG="15.8.1.085"
PG15_INITDB_TAG="15-alpine"

# Pod names for temporary rollback pods
SWAP_POD="supabase-rollback-swap"
FIX_POD="supabase-rollback-fix-pgsodium"

# PVC and StatefulSet names (detected during preflight)
DB_STS_NAME=""
DB_POD_NAME=""
DB_PVC_NAME=""
PGSODIUM_PVC_NAME=""
PG_PASSWORD=""
CURRENT_IMAGE=""
CURRENT_TAG=""

# The Helm chart uses subPath: postgres-data inside the PVC.
DATA_SUBPATH="postgres-data"
BACKUP_SUBPATH="postgres-data.bak.pg15"

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
    kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    kubectl delete pod "$FIX_POD" -n "$NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
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

# --- Pre-flight checks -----------------------------------------------------

preflight() {
    info "Running pre-flight checks"

    # Check required tools
    command -v kubectl >/dev/null 2>&1 || die "kubectl not found."
    command -v helm >/dev/null 2>&1 || die "helm not found."

    # Check namespace exists
    kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 \
        || die "Namespace '$NAMESPACE' not found."

    # Check Helm release exists
    helm status "$RELEASE" -n "$NAMESPACE" >/dev/null 2>&1 \
        || die "Helm release '$RELEASE' not found in namespace '$NAMESPACE'."

    # Auto-detect chart path from release if not provided.
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

    # Verify the pod is actually running PG 17
    local pg_version
    pg_version=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        psql -h localhost -U postgres -At -c "SHOW server_version;" 2>/dev/null | head -n 1) || true
    echo "  Postgres version: $pg_version"
    case "$pg_version" in
        17.*) ;;
        15.*) warn "Already running Postgres 15. Nothing to roll back."; exit 0 ;;
        "") die "Could not query Postgres version. Is the DB pod healthy?" ;;
        *) die "Unexpected Postgres version: $pg_version (expected 17.x)" ;;
    esac
}

# --- Show summary and confirm -----------------------------------------------

show_summary() {
    echo ""
    echo "This script will:"
    echo "  1. Scale down all Supabase services for release '$RELEASE'"
    echo "  2. Remove the Postgres 17 data directory (postgres-data)"
    echo "  3. Restore the Postgres 15 backup directory (postgres-data.bak.pg15)"
    echo "  4. Fix pgsodium volume ownership for the PG 15 image"
    echo "  5. Roll the Helm release back to Postgres 15 image tags"
    echo "  6. Scale up the DB StatefulSet"
    echo "  7. Verify Postgres is version 15"
    echo ""
    echo "  Namespace:        $NAMESPACE"
    echo "  Helm release:     $RELEASE"
    echo "  StatefulSet:      $DB_STS_NAME"
    echo "  Current image:    $CURRENT_IMAGE"
    echo "  Target image:     $PG15_TARGET_IMAGE"
    echo "  DB PVC:           $DB_PVC_NAME"
    echo "  Pgsodium PVC:     $PGSODIUM_PVC_NAME"
    echo "  Data subpath:     $DATA_SUBPATH"
    echo "  Backup subpath:   $BACKUP_SUBPATH"
    echo ""
    warn "This is DESTRUCTIVE and will remove the Postgres 17 data directory."
    warn "Ensure you have a snapshot or backup of the DB PVC before proceeding."
    echo ""
    confirm "Proceed with the rollback?"
}

# --- Step 1: Scale down all services ----------------------------------------

scale_down_all() {
    info "Scaling down all Supabase services"

    local deployments statefulsets

    deployments=$(kubectl get deployment -n "$NAMESPACE" \
        -l "app.kubernetes.io/instance=$RELEASE" \
        -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.replicas}{"\n"}{end}' 2>/dev/null) || true

    statefulsets=$(kubectl get statefulset -n "$NAMESPACE" \
        -l "app.kubernetes.io/instance=$RELEASE" \
        -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.replicas}{"\n"}{end}' 2>/dev/null) || true

    # Scale down deployments first
    while IFS=' ' read -r name replicas; do
        [ -z "$name" ] && continue
        echo "  Scaling down deployment/$name (was: ${replicas:-1} replicas)"
        kubectl scale deployment -n "$NAMESPACE" "$name" --replicas=0 >/dev/null 2>&1 || true
    done <<< "$deployments"

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

# --- Step 2: Swap data directories inside PVC --------------------------------

swap_data_back() {
    info "Swapping data directories inside PVC"

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

    # Verify the backup directory exists inside the full PVC mount.
    local has_backup
    has_backup=$(kubectl exec -n "$NAMESPACE" "$SWAP_POD" -- \
        sh -c "[ -d '/mnt/db-data/$BACKUP_SUBPATH' ] && echo yes || echo no" 2>/dev/null)
    if [ "$has_backup" != "yes" ]; then
        kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
        die "Backup directory '$BACKUP_SUBPATH' not found inside the DB PVC. Cannot roll back."
    fi
    echo "  PG 15 backup found: $BACKUP_SUBPATH"

    if ! kubectl exec -n "$NAMESPACE" "$SWAP_POD" -- sh -c "
        set -e
        echo 'Removing Postgres 17 data directory...'
        rm -rf /mnt/db-data/$DATA_SUBPATH

        echo 'Restoring Postgres 15 backup...'
        mv /mnt/db-data/$BACKUP_SUBPATH /mnt/db-data/$DATA_SUBPATH

        echo 'Final PVC contents:'
        ls -la /mnt/db-data/
    "; then
        kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
        die "Data swap failed."
    fi

    kubectl delete pod "$SWAP_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
}

# --- Step 3: Fix pgsodium ownership for PG 15 --------------------------------

fix_pgsodium_ownership() {
    info "Fixing pgsodium volume ownership for PG 15"

    # The pgsodium volume was last owned by the PG 17 image's postgres user.
    # We must re-chown it using the PG 15 image so the UID/GID matches.
    kubectl run "$FIX_POD" -n "$NAMESPACE" \
        --image="$PG15_TARGET_IMAGE" \
        --restart=Never \
        --overrides="{
            \"apiVersion\": \"v1\",
            \"spec\": {
                \"containers\": [{
                    \"name\": \"fix-pgsodium\",
                    \"image\": \"$PG15_TARGET_IMAGE\",
                    \"command\": [\"sleep\", \"infinity\"],
                    \"volumeMounts\": [{\"name\": \"pgsodium\", \"mountPath\": \"/vol\"}]
                }],
                \"volumes\": [{\"name\": \"pgsodium\", \"persistentVolumeClaim\": {\"claimName\": \"$PGSODIUM_PVC_NAME\"}}],
                \"restartPolicy\": \"Never\"
            }
        }" >/dev/null 2>&1

    wait_for_pod_ready "$FIX_POD" 120

    kubectl exec -n "$NAMESPACE" "$FIX_POD" -- sh -c \
        'mkdir -p /vol/conf.d && chown -R postgres:postgres /vol/ && echo "pgsodium ownership fixed for PG 15"' \
        || die "pgsodium ownership fix failed."

    kubectl delete pod "$FIX_POD" -n "$NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
}

# --- Step 4: Helm rollback to PG 15 ------------------------------------------

helm_rollback() {
    info "Rolling Helm release back to Postgres 15"

    local helm_args=(
        upgrade "$RELEASE"
        --namespace "$NAMESPACE"
        --set "image.db.tag=$PG15_TARGET_TAG"
        --set "image.initDb.tag=$PG15_INITDB_TAG"
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
            echo "      --set image.db.tag=$PG15_TARGET_TAG \\"
            echo "      --set image.initDb.tag=$PG15_INITDB_TAG \\"
            echo "      --reuse-values"
            echo ""
            confirm "Skip automatic helm rollback and continue with manual instructions?"
            return
        fi
    else
        echo ""
        echo "  Could not determine chart path automatically."
        echo "  Please run the helm upgrade manually:"
        echo ""
        echo "    helm upgrade $RELEASE <chart-path> \\"
        echo "      --namespace $NAMESPACE \\"
        echo "      --set image.db.tag=$PG15_TARGET_TAG \\"
        echo "      --set image.initDb.tag=$PG15_INITDB_TAG \\"
        echo "      --reuse-values"
        echo ""
        confirm "Skip automatic helm rollback and continue with manual instructions?"
        return
    fi

    echo "  Running: helm ${helm_args[*]}"
    helm "${helm_args[@]}"
}

# --- Step 5: Scale up DB and verify ------------------------------------------

scale_up_db() {
    info "Scaling up DB StatefulSet"

    kubectl scale statefulset -n "$NAMESPACE" "$DB_STS_NAME" --replicas=1 >/dev/null 2>&1 || true

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
    [ $retries -gt 0 ] || die "DB pod did not start within 180 seconds after helm rollback."

    wait_for_pg_ready "$DB_POD_NAME" 60
}

verify_pg15() {
    info "Verifying Postgres 15"

    local version
    version=$(kubectl exec -n "$NAMESPACE" "$DB_POD_NAME" -- \
        psql -h localhost -U supabase_admin -d postgres -At \
        -c "SHOW server_version;" 2>/dev/null | head -n 1)
    echo "  Postgres version: $version"

    case "$version" in
        15.*) ;;
        *) die "Expected Postgres 15.x, got: $version" ;;
    esac

    info "Rollback complete!"
    echo ""
    echo "  Postgres 15 is running in namespace '$NAMESPACE'."
    echo "  Helm release '$RELEASE' has been rolled back to:"
    echo "    image.db.tag=$PG15_TARGET_TAG"
    echo "    image.initDb.tag=$PG15_INITDB_TAG"
    echo ""
    echo "  The Postgres 17 data directory has been removed."
    echo "  If you want to upgrade again later, re-run scripts/upgrade-pg17.sh."
    echo ""
}

# --- Main -------------------------------------------------------------------

main() {
    echo ""
    echo "Supabase Kubernetes: Postgres 17 -> 15 Rollback"
    echo "================================================"

    preflight
    show_summary
    scale_down_all
    swap_data_back
    fix_pgsodium_ownership
    helm_rollback
    scale_up_db
    verify_pg15
}

main "$@"
