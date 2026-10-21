#!/usr/bin/env bash
#
# Applies the configuration defaults that belong to an installed pack.
#
# One deployment.yaml serves three deployments: the container image, the Helm
# chart, and the pack. Most of it is the same for all three, but the datasource
# is not. A container is handed a PostgreSQL to use; a pack is extracted onto a
# machine where nothing else has been installed, and a default pointing at a
# database that is not there means it cannot start at all.
#
# So the packed copy — and only the packed copy — defaults to the inbuilt
# database. Nothing the container or the chart reads is touched.
#
#   pack-config.sh <deployment.yaml>

set -euo pipefail

CONFIG="${1:?usage: pack-config.sh <deployment.yaml>}"
[ -f "$CONFIG" ] || { echo "pack-config: $CONFIG not found" >&2; exit 1; }

# Scoped to the datasource block: `type` appears under message_queue too, and
# rewriting that one would change which queue provider the pack uses.
awk '
  /^[^[:space:]#]/ { block = $1 }
  block == "datasource:" && /^[[:space:]]+type:/ {
    sub(/type:.*/, "type: \"sqlite\" # The pack defaults to the inbuilt database; see the README for PostgreSQL.")
  }
  { print }
' "$CONFIG" > "$CONFIG.packed"

grep -q 'type: "sqlite"' "$CONFIG.packed" || {
  echo "pack-config: the datasource default was not applied to $CONFIG" >&2
  rm -f "$CONFIG.packed"
  exit 1
}
mv "$CONFIG.packed" "$CONFIG"
