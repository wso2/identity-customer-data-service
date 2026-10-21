#!/usr/bin/env bash
#
# Attaches the WSO2 update tool setup script to built CDS packs.
#
# The product build does not stage this script, deliberately. It is published
# and signed by the Updates team rather than built from this source — the same
# split product-is uses, where updates/product.txt is part of the product build
# and the setup script is attached when the pack is published.
#
#   attach-update-tool.sh <dist-dir> <update_tool_setup.zip>
#
# For each <dist-dir>/*.zip it unpacks the pack, installs
# bin/update_tool_setup.sh with the mode the tool expects, re-zips it in place
# and rewrites the checksums beside it, so what is published is what the
# checksum describes.

set -euo pipefail

DIST_DIR="${1:?usage: attach-update-tool.sh <dist-dir> <update_tool_setup.zip>}"
SETUP_ZIP="${2:?usage: attach-update-tool.sh <dist-dir> <update_tool_setup.zip>}"

[ -d "$DIST_DIR" ]   || { echo "attach-update-tool: $DIST_DIR not found" >&2; exit 1; }
[ -f "$SETUP_ZIP" ]  || { echo "attach-update-tool: $SETUP_ZIP not found" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

unzip -q "$SETUP_ZIP" -d "$WORK/setup"
SETUP_SH="$(find "$WORK/setup" -name 'update_tool_setup.sh' -type f | head -1)"
[ -n "$SETUP_SH" ] || { echo "attach-update-tool: $SETUP_ZIP has no update_tool_setup.sh" >&2; exit 1; }

checksum() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1"; else shasum -a 256 "$1"; fi
}

attached=0
for zip in "$DIST_DIR"/*.zip; do
  [ -e "$zip" ] || continue
  pack="$(basename "$zip" .zip)"

  rm -rf "$WORK/pack"
  mkdir -p "$WORK/pack"
  unzip -q "$zip" -d "$WORK/pack"

  # bin/ and updates/ have to stay siblings at the pack root: the setup script
  # resolves ../updates/product.txt relative to its own location.
  [ -f "$WORK/pack/$pack/updates/product.txt" ] \
    || { echo "attach-update-tool: $pack carries no updates/product.txt" >&2; exit 1; }

  install -m 755 "$SETUP_SH" "$WORK/pack/$pack/bin/update_tool_setup.sh"

  rm -f "$zip"
  (cd "$WORK/pack" && zip -rq "$zip" "$pack")
  checksum "$zip" > "$zip.sha256"
  if command -v md5sum >/dev/null 2>&1; then md5sum "$zip" > "$zip.md5"; else md5 -r "$zip" > "$zip.md5"; fi

  echo "  -> $pack carries bin/update_tool_setup.sh ($(cat "$WORK/pack/$pack/updates/product.txt"))"
  attached=$((attached + 1))
done

[ "$attached" -gt 0 ] || { echo "attach-update-tool: no packs in $DIST_DIR" >&2; exit 1; }
echo "Attached the update tool setup script to $attached pack(s)."
