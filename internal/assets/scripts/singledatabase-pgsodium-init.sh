#!/bin/sh
set -e

PGSODIUM_DIR="/mnt/pgsodium"

# If the pgsodium directory is empty, copy the default
# /etc/postgresql-custom files from the image into the PVC
if [ -z "$(ls -A "$PGSODIUM_DIR" 2>/dev/null)" ]; then
  echo "pgsodium volume is empty, copying default /etc/postgresql-custom files..."
  cp -a /etc/postgresql-custom/. "$PGSODIUM_DIR/"
else
  echo "pgsodium volume already initialized, skipping copy"
fi
