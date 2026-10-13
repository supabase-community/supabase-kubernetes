#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rendered_manifest="$(mktemp)"
trap 'rm -f "${rendered_manifest}"' EXIT

helm template supabase "${repository_root}/charts/supabase" \
  --namespace default \
  --kube-version 1.31.0 \
  --skip-tests \
  --show-only templates/storage/deployment.yaml \
  --set deployment.minio.enabled=true \
  --set deployment.storage.enabled=true \
  --set deployment.storage.initDb=true \
  --set deployment.storage.securityContext.allowPrivilegeEscalation=false \
  --set deployment.storage.securityContext.privileged=false \
  --set deployment.storage.securityContext.readOnlyRootFilesystem=true \
  --set deployment.storage.securityContext.runAsNonRoot=true \
  --set deployment.storage.securityContext.runAsUser=10001 \
  --set deployment.storage.securityContext.seccompProfile.type=RuntimeDefault \
  > "${rendered_manifest}"

assert_security_context() {
  local container_name="$1"

  if ! awk -v target="${container_name}" '
    /^        - name: / {
      if (in_target) {
        exit
      }
      if ($0 == "        - name: " target) {
        in_target = 1
        found = 1
      }
      next
    }
    in_target && /^      containers:/ { exit }
    in_target && /^          securityContext:$/ { context = 1 }
    in_target && /^            allowPrivilegeEscalation: false$/ { allow = 1 }
    in_target && /^            privileged: false$/ { privileged = 1 }
    in_target && /^            readOnlyRootFilesystem: true$/ { readonly = 1 }
    in_target && /^            runAsNonRoot: true$/ { nonroot = 1 }
    in_target && /^            runAsUser: 10001$/ { user = 1 }
    in_target && /^              type: RuntimeDefault$/ { seccomp = 1 }
    END {
      if (!(found && context && allow && privileged && readonly && nonroot && user && seccomp)) {
        exit 1
      }
    }
  ' "${rendered_manifest}"; then
    echo "Expected propagated Storage securityContext on ${container_name}" >&2
    return 1
  fi
}

assert_security_context init-db
assert_security_context init-bucket
