#!/usr/bin/env bash
#
# Writes the release body that goes above GitHub's generated change list.
#
# A release publishes seven files. Landing on that page, the questions are
# always the same: which one do I want, is it the file you built, and what do
# I do with it. The generated "What's Changed" list answers none of them, so
# this answers them first — from the artifacts themselves, never from a list
# kept by hand that would drift from what was built.
#
#   release-notes.sh <dist-dir> [version] [--changelog <file>]
#
# The change list is GitHub's, generated from the merged pull requests and
# grouped by .github/release.yml. It is passed in rather than fetched here, so
# this script stays usable from a working copy and on a pull request, where
# there is no release to generate one for.
#
# Called by the release workflow, which passes the result as the release body.

set -euo pipefail

DIST_DIR="${1:?usage: release-notes.sh <dist-dir> [version] [--changelog <file>]}"
shift

VERSION=""
CHANGELOG=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changelog) CHANGELOG="${2:?--changelog requires a file}"; shift 2 ;;
    *)           VERSION="$1"; shift ;;
  esac
done
[ -n "$VERSION" ] || VERSION="$(tr -d '[:space:]' < version.txt)"

PACK_VERSION="${VERSION#v}"
PRODUCT="wso2cds"
REPO="${CDS_REPO:-wso2/identity-customer-data-service}"

[ -d "$DIST_DIR" ] || { echo "release-notes: $DIST_DIR not found" >&2; exit 1; }

METADATA="$DIST_DIR/$PRODUCT-$PACK_VERSION-release-metadata.yaml"

# The checksum files are what an installation verifies against, so the digests
# shown here are read from those rather than recomputed — if the two ever
# disagreed, this would be the wrong place to find out.
digest_of() {
  local file="$1.sha256"
  [ -f "$file" ] || { echo "unavailable"; return; }
  awk '{print $1; exit}' "$file"
}

# The extension baseline, which lives under compatibility.extensions.
#
# Scoped to that block rather than matched on the key alone: the metadata also
# carries a top-level `version` for the release itself, and reporting that as
# the extension baseline would be worse than reporting nothing.
extension_version() {
  [ -f "$METADATA" ] || return 0
  awk '
    /^[[:space:]]*extensions:[[:space:]]*$/ { inside = 1; next }
    inside && /^[[:space:]]*[a-z_]+:/ && !/^[[:space:]]*(version|repository|group|bundles):/ { inside = 0 }
    inside && /^[[:space:]]*version:/ {
      sub(/^[^:]*:[[:space:]]*/, ""); gsub(/"/, ""); print; exit
    }
  ' "$METADATA"
}

supported_is() {
  [ -f "$METADATA" ] || return 0
  awk '
    /^[[:space:]]*identity_server:/ { inside = 1; next }
    inside && /^[[:space:]]*[a-z_]+:[[:space:]]*$/ && !/supported/ { inside = 0 }
    inside && /^[[:space:]]*-[[:space:]]/ { gsub(/^[[:space:]]*-[[:space:]]*"?|"?[[:space:]]*$/, ""); print }
    inside && /supported:[[:space:]]*\[.+\]/ {
      sub(/^[^[]*\[/, ""); sub(/\].*$/, ""); gsub(/[", ]+/, "\n"); print
    }
  ' "$METADATA" | grep -v '^$' | tr '\n' ',' | sed -e 's/,$//' -e 's/,/, /g'
}

# --- Introduction --------------------------------------------------------- #
# GitHub already shows the release name above this, so the body opens with
# what the reader cannot see from the title.
cat <<INTRO
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Release](https://img.shields.io/github/v/release/$REPO?color=blue)](https://github.com/$REPO/releases/latest)
[![Last commit](https://img.shields.io/github/last-commit/$REPO.svg)](https://github.com/$REPO/commits/main)

The WSO2 Customer Data Service builds unified customer profiles from the
identity and behavioural data an organisation already holds, and serves them
to the applications that personalise on them.

CDS runs as its own process beside a WSO2 Identity Server. The two are
separate installations with separate lifecycles: CDS is installed, enabled and
updated on its own, and the Identity Server is never started, stopped or
updated by it.

INTRO

# --- Downloads ------------------------------------------------------------ #
echo "## Downloads"
echo
echo "| Platform | Artifact | SHA-256 |"
echo "|---|---|---|"

found=0
for zip in "$DIST_DIR"/$PRODUCT-*-"$PACK_VERSION".zip; do
  [ -e "$zip" ] || continue
  name="$(basename "$zip")"
  # wso2cds-<os>-<arch>-<version>.zip
  platform="$(echo "$name" | sed -E "s/^$PRODUCT-([a-z0-9]+)-([a-z0-9]+)-.*/\1\/\2/")"
  printf '| %s | [%s](https://github.com/%s/releases/download/%s/%s) | `%s` |\n' \
    "$platform" "$name" "$REPO" "$VERSION" "$name" "$(digest_of "$zip")"
  found=$((found + 1))
done
[ "$found" -gt 0 ] || { echo "release-notes: no packs in $DIST_DIR" >&2; exit 1; }

echo
echo "Each zip is published with a \`.sha256\` and \`.md5\` beside it:"
echo
echo '```bash'
echo "sha256sum -c $PRODUCT-linux-amd64-$PACK_VERSION.zip.sha256"
echo '```'
echo

# --- Compatibility -------------------------------------------------------- #
IS_SUPPORTED="$(supported_is || true)"
EXTENSION="$(extension_version || true)"
echo "## Compatibility"
echo
if [ -n "$IS_SUPPORTED" ]; then
  echo "| | |"
  echo "|---|---|"
  echo "| Identity Server | $IS_SUPPORTED |"
  [ -n "$EXTENSION" ] && echo "| CDS extension baseline | $EXTENSION |"
else
  echo "This build declares no verified Identity Server baseline. \`wso2cds enable\`"
  echo "says so and continues, rather than claiming a compatibility that has not"
  echo "been tested."
fi
echo
echo "\`$PRODUCT-$PACK_VERSION-release-metadata.yaml\` carries the full statement, and is what"
echo "the bootstrap installer reads to resolve a release for an installed Identity Server."
echo

# --- Installing ----------------------------------------------------------- #
cat <<NOTES
## Installing

Starting from CDS — extraction is the installation, there is nothing to run
first:

\`\`\`bash
unzip $PRODUCT-linux-amd64-$PACK_VERSION.zip -d /opt
cd /opt/$PRODUCT-linux-amd64-$PACK_VERSION
./bin/$PRODUCT enable --is-home /opt/wso2is
./bin/$PRODUCT start && ./bin/$PRODUCT verify
\`\`\`

Starting from an Identity Server that is already installed, the bootstrap
installer resolves the right pack for it:

\`\`\`bash
./install_cds.sh --is-home /opt/wso2is
\`\`\`

Both paths end at the same installation and the same enable flow. See
[On-Prem Installation](https://github.com/$REPO/blob/$VERSION/docs/guides/on-prem-installation.md)
for what enablement does on each side of the Identity Server restart.

The CDS extension bundles and the CDS Console are delivered with the Identity
Server rather than with this release, and are updated through its own stream.
NOTES

# --- What changed ---------------------------------------------------------- #
if [ -n "$CHANGELOG" ] && [ -s "$CHANGELOG" ]; then
  echo
  cat "$CHANGELOG"
fi

# --- License --------------------------------------------------------------- #
cat <<'LICENCE'

## License

Licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/wso2/identity-customer-data-service/blob/main/LICENSE).
The third-party inventory for the binaries in this release is in `LICENSE.txt`
inside each pack.
LICENCE
