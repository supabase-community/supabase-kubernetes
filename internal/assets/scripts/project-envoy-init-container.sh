#!/bin/sh
set -e

CONFIG_SOURCE="/etc/envoy-config"
CONFIG_TARGET="/etc/envoy"

echo "Copying Envoy configuration from ConfigMap..."
cp "${CONFIG_SOURCE}/envoy.yaml" "${CONFIG_TARGET}/envoy.yaml"
cp "${CONFIG_SOURCE}/cds.yaml" "${CONFIG_TARGET}/cds.yaml"
cp "${CONFIG_SOURCE}/lds.template.yaml" "${CONFIG_TARGET}/lds.template.yaml"
cp "${CONFIG_SOURCE}/docker-entrypoint.sh" "${CONFIG_TARGET}/docker-entrypoint.sh"
