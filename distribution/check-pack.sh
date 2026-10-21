#!/usr/bin/env bash
#
# Checks that every built pack carries what an installation needs.
#
# A pack that quietly lost a file still builds, still zips and still uploads;
# the loss only shows up when a customer extracts it. This runs on every pull
# request so that a change to the packaging is caught where it was made.
#
#   check-pack.sh <dist-dir>

set -euo pipefail

DIST_DIR="${1:?usage: check-pack.sh <dist-dir>}"
[ -d "$DIST_DIR" ] || { echo "check-pack: $DIST_DIR not found" >&2; exit 1; }

# Paths every pack must contain, relative to the pack root. bin/wso2cds is the
# product; the rest is what it reads on first start, plus the identity the
# update service resolves the pack by.
REQUIRED="
bin/wso2cds
repository/conf/deployment.yaml
repository/conf/compatibility.yaml
repository/dbscripts/postgres.sql
repository/dbscripts/sqlite.sql
repository/resources/security/README.md
updates/product.txt
version.txt
LICENSE.txt
README.md
"

failures=0
packs=0

for zip in "$DIST_DIR"/*.zip; do
  [ -e "$zip" ] || continue
  pack="$(basename "$zip" .zip)"
  packs=$((packs + 1))
  listing="$(unzip -Z1 "$zip")"

  for required in $REQUIRED; do
    if ! printf '%s\n' "$listing" | grep -qx "$pack/$required"; then
      echo "  x $pack is missing $required" >&2
      failures=$((failures + 1))
    fi
  done

  # The product identity has to name the pack it sits in, or the update
  # service resolves updates for a different platform.
  identity="$(unzip -p "$zip" "$pack/updates/product.txt" 2>/dev/null | tr -d '[:space:]')"
  if [ "$identity" != "$pack" ]; then
    echo "  x $pack carries the product identity '$identity'" >&2
    failures=$((failures + 1))
  fi

  [ "$failures" -eq 0 ] && echo "  + $pack"
done

if [ "$packs" -eq 0 ]; then
  echo "check-pack: no packs in $DIST_DIR" >&2
  exit 1
fi
if [ "$failures" -gt 0 ]; then
  echo "check-pack: $failures problem(s) across $packs pack(s)" >&2
  exit 1
fi
echo "check-pack: $packs pack(s) carry every required file."
