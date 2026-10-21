#!/usr/bin/env bash
#
# Drives the real bootstrap installer against packs that were just built.
#
# install_cds.sh is the first thing a customer on the Identity-Server-first
# path runs, and until this existed nothing exercised it: a change that broke
# release resolution, the checksum check or the hand-off to `wso2cds enable`
# would have been found by them rather than by us. So this stands a release up
# locally — the artifacts served over HTTP, an Identity Server installation to
# enable against — and runs the installer end to end against it.
#
#   verify-install.sh <dist-dir>
#
# Linux only, because that is what CDS is released for and what the installer
# accepts. Needs: curl, unzip, python3.

set -euo pipefail

DIST_DIR="${1:?usage: verify-install.sh <dist-dir>}"
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRODUCT="wso2cds"

# The release this fixture pretends to be. The Identity Server version is the
# one the metadata will declare support for, so the positive branch of release
# resolution is what runs rather than the fallback.
IS_VERSION="${VERIFY_IS_VERSION:-7.4.0}"

if [ "$(uname -s)" != "Linux" ]; then
  echo "verify-install: skipped, CDS is released for Linux only (this is $(uname -s))" >&2
  exit 0
fi

for binary in curl unzip python3; do
  command -v "$binary" >/dev/null 2>&1 || { echo "verify-install: $binary is required" >&2; exit 1; }
done

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "verify-install: unsupported architecture $ARCH" >&2; exit 1 ;;
esac

VERSION="$(tr -d '[:space:]' < version.txt)"
PACK_VERSION="${VERSION#v}"
PACK="$PRODUCT-linux-$ARCH-$PACK_VERSION"
[ -f "$DIST_DIR/$PACK.zip" ] || { echo "verify-install: $DIST_DIR/$PACK.zip was not built" >&2; exit 1; }

WORK="$(mktemp -d)"
SERVER_PID=""
cleanup() {
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    # Waiting on it keeps the shell from announcing the job it just killed,
    # which otherwise lands in the log looking like a failure.
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

# --------------------------------------------------------------------------- #
# A release, served locally
#
# The layout is the one the releases have — the API listing the installer reads
# to find candidate tags, and <tag>/<file> for the artifacts — so the installer
# runs its real resolution path rather than a shortcut taken for the test.
# --------------------------------------------------------------------------- #
WWW="$WORK/www"
mkdir -p "$WWW/repos/wso2/identity-customer-data-service" "$WWW/$VERSION"

printf '[{"tag_name":"%s","draft":false,"prerelease":false}]\n' "$VERSION" \
  > "$WWW/repos/wso2/identity-customer-data-service/releases"

cp "$DIST_DIR/$PACK.zip" "$DIST_DIR/$PACK.zip.sha256" "$WWW/$VERSION/"

# The generated metadata declares no verified baseline on a development build,
# which would send the installer down its fallback branch. Declaring one here
# is what puts the match on trial.
METADATA="$PRODUCT-$PACK_VERSION-release-metadata.yaml"
sed "s/^    supported: \[\]/    supported:\n      - \"$IS_VERSION\"/" \
  "$DIST_DIR/$METADATA" > "$WWW/$VERSION/$METADATA"
grep -q "$IS_VERSION" "$WWW/$VERSION/$METADATA" \
  || { echo "verify-install: the metadata fixture does not declare $IS_VERSION" >&2; exit 1; }

# A port is picked rather than fixed, because a fixed one collides with
# whatever else the host is running and turns this into a flaky check.
PORT="${VERIFY_PORT:-0}"
if [ "$PORT" = "0" ]; then
  PORT="$(python3 -c 'import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()')"
fi

( cd "$WWW" && python3 -m http.server "$PORT" --bind 127.0.0.1 >/dev/null 2>&1 ) &
SERVER_PID=$!

ready=0
for _ in $(seq 1 50); do
  if curl -fsS -o /dev/null "http://127.0.0.1:$PORT/$VERSION/$PACK.zip.sha256" 2>/dev/null; then
    ready=1
    break
  fi
  sleep 0.2
done
[ "$ready" = "1" ] || { echo "verify-install: the local artifact server did not start on port $PORT" >&2; exit 1; }

# --------------------------------------------------------------------------- #
# An Identity Server to enable against
#
# Only the parts enablement reads: the file that identifies it as an Identity
# Server, its version, and the dropins. The extension bundles are put there as
# a release that ships CDS support would, which is also what keeps this from
# reaching the Maven repository.
# --------------------------------------------------------------------------- #
IS_HOME="$WORK/wso2is"
mkdir -p "$IS_HOME/repository/conf" "$IS_HOME/bin" "$IS_HOME/updates" \
         "$IS_HOME/repository/components/dropins"

printf '[server]\nhostname = "localhost"\n' > "$IS_HOME/repository/conf/deployment.toml"
printf '#!/bin/sh\n' > "$IS_HOME/bin/wso2server.sh"
chmod +x "$IS_HOME/bin/wso2server.sh"
printf 'WSO2 Identity Server v%s\n' "$IS_VERSION" > "$IS_HOME/bin/version.txt"
printf 'wso2is-%s' "$IS_VERSION" > "$IS_HOME/updates/product.txt"
for bundle in client event.handler auth.listener auth.handler; do
  : > "$IS_HOME/repository/components/dropins/org.wso2.identity.customer.data.service.$bundle-0.9.0.jar"
done

# --------------------------------------------------------------------------- #
# Run it
#
# Nothing is stubbed past this point. The installer resolves the release from
# the metadata, verifies the checksum, extracts, and execs the packed
# wso2cds enable, which reads the Identity Server and stages its configuration.
# Provisioning stops at the restart this fixture will never perform, which is
# the correct outcome and the one an operator sees.
# --------------------------------------------------------------------------- #
CDS_HOME="$WORK/wso2cds"
echo "verify-install: installing $PACK against Identity Server $IS_VERSION"

CDS_API="http://127.0.0.1:$PORT" \
CDS_DOWNLOAD_BASE="http://127.0.0.1:$PORT" \
CDS_IS_ADMIN_PASSWORD="verify-install-not-used" \
  "$SELF_DIR/install_cds.sh" \
    --is-home "$IS_HOME" \
    --cds-home "$CDS_HOME" \
    --accept-license \
  > "$WORK/install.log" 2>&1 || {
    echo "verify-install: the installer failed" >&2
    cat "$WORK/install.log" >&2
    exit 1
  }

# --------------------------------------------------------------------------- #
# What has to be true afterwards
# --------------------------------------------------------------------------- #
failures=0
check() {
  if eval "$2" >/dev/null 2>&1; then
    echo "  + $1"
  else
    echo "  x $1" >&2
    failures=$((failures + 1))
  fi
}

check "the release was resolved from its declared Identity Server support" \
  "grep -q 'declares support for Identity Server $IS_VERSION' '$WORK/install.log'"
check "the download was checked against its published checksum" \
  "grep -q 'checksum verified' '$WORK/install.log'"
check "CDS_HOME holds the pack" "[ -x '$CDS_HOME/bin/$PRODUCT' ]"
check "the installed executable reports the built version" \
  "[ \"\$('$CDS_HOME/bin/$PRODUCT' version)\" = '$PACK_VERSION' ]"
check "the installer handed off to wso2cds enable" \
  "grep -q 'Identity Server restart required' '$WORK/install.log'"
check "enablement recorded the Identity Server" \
  "grep -q 'is_version: $IS_VERSION' '$CDS_HOME/repository/conf/integration.yaml'"
check "the bundles the Identity Server provides were left to it" \
  "grep -q 'provided by the Identity Server' '$WORK/install.log'"
check "nothing was installed over them" \
  "[ \"\$(ls -1 '$IS_HOME/repository/components/dropins' | wc -l | tr -d '[:space:]')\" = '4' ]"
check "the Identity Server configuration was staged" \
  "grep -q '# BEGIN wso2cds' '$IS_HOME/repository/conf/deployment.toml'"

if [ "$failures" -gt 0 ]; then
  echo
  echo "verify-install: $failures check(s) failed" >&2
  echo "--- installer output ---" >&2
  cat "$WORK/install.log" >&2
  exit 1
fi

echo "verify-install: the Identity-Server-first path installs and enables $PACK."
