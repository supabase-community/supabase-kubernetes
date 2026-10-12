#!/bin/bash

set -euo pipefail

# Export empty optional keys so awk removes their placeholders in legacy mode.
export SUPABASE_PUBLISHABLE_KEY="${SUPABASE_PUBLISHABLE_KEY:-}"
export SUPABASE_SECRET_KEY="${SUPABASE_SECRET_KEY:-}"

if [ -n "${SUPABASE_SECRET_KEY:-}" ] && [ -n "${SUPABASE_PUBLISHABLE_KEY:-}" ]; then
  export LUA_AUTH_EXPR="\$((headers.authorization ~= nil and headers.authorization:sub(1, 10) ~= 'Bearer sb_' and headers.authorization) or (headers.apikey == '${SUPABASE_SECRET_KEY}' and 'Bearer ${SERVICE_ROLE_KEY_ASYMMETRIC}') or (headers.apikey == '${SUPABASE_PUBLISHABLE_KEY}' and 'Bearer ${ANON_KEY_ASYMMETRIC}') or headers.apikey)"
  export LUA_RT_WS_EXPR="\$((query_params.apikey == '${SUPABASE_SECRET_KEY}' and '${SERVICE_ROLE_KEY_ASYMMETRIC}') or (query_params.apikey == '${SUPABASE_PUBLISHABLE_KEY}' and '${ANON_KEY_ASYMMETRIC}') or query_params.apikey)"
else
  export LUA_AUTH_EXPR="\$((headers.authorization ~= nil and headers.authorization:sub(1, 10) ~= 'Bearer sb_' and headers.authorization) or headers.apikey)"
  export LUA_RT_WS_EXPR="\$(query_params.apikey)"
fi

echo "Replacing env placeholders of ${KONG_DECLARATIVE_CONFIG}"

# Substitute once, preserving YAML quoting, Lua expressions and literal values
# containing sed replacement characters. Accept ${VAR} for existing ConfigMaps
# as well as the upstream $VAR syntax. Leave unknown variables unchanged.
awk '{
  result = ""
  rest = $0
  while (match(rest, /\$[A-Za-z_][A-Za-z_0-9]*|\$\{[A-Za-z_][A-Za-z_0-9]*\}/)) {
    varname = substr(rest, RSTART + 1, RLENGTH - 1)
    if (substr(varname, 1, 1) == "{") {
      varname = substr(varname, 2, length(varname) - 2)
    }
    if (varname in ENVIRON) {
      result = result substr(rest, 1, RSTART - 1) ENVIRON[varname]
    } else {
      result = result substr(rest, 1, RSTART + RLENGTH - 1)
    }
    rest = substr(rest, RSTART + RLENGTH)
  }
  print result rest
}' /usr/local/kong/template.yml > "$KONG_DECLARATIVE_CONFIG"

sed -i '/^[[:space:]]*- key:[[:space:]]*$/d' "$KONG_DECLARATIVE_CONFIG"

exec /entrypoint.sh kong docker-start
