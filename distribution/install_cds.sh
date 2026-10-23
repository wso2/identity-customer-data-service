#!/usr/bin/env bash
#
# Installs the Customer Data Service beside an existing WSO2 Identity Server.
#
# This is the IS-first onboarding path. The customer already runs an Identity
# Server and decides to add CDS, so the CDS executable does not exist yet and
# something has to acquire it — that is all this script is. It resolves a CDS
# release compatible with the installed Identity Server, downloads and extracts
# the pack for this platform, and hands over to the packaged `cds enable`.
#
# Everything after the hand-off is the product's own command, so this script
# and the CDS-first path (download the zip, extract it, run cds enable)
# converge on one implementation rather than two.
#
#   ./install_cds.sh --is-home /opt/wso2is [--cds-home /opt/cds]
#
# Needs: curl, unzip, and a writable CDS_HOME.

set -euo pipefail

REPO="${CDS_REPO:-wso2/identity-customer-data-service}"
API="${CDS_API:-https://api.github.com}"
# Where the release artifacts are fetched from. An installation behind a proxy,
# or one served from an internal mirror of the releases, points this at that
# mirror; the layout it expects is the one the releases have, <tag>/<file>.
DOWNLOAD_BASE="${CDS_DOWNLOAD_BASE:-https://github.com/$REPO/releases/download}"
PRODUCT="cds"

IS_HOME=""
CDS_HOME=""
VERSION=""
ACCEPT_LICENSE="0"
RUN_ENABLE="1"

C_BOLD=""; C_RESET=""; C_RED=""; C_GREEN=""; C_YELLOW=""
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_BOLD=$'\033[1m'; C_RESET=$'\033[0m'; C_RED=$'\033[31m'
  C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'
fi

step() { printf '\n%s%s%s\n' "$C_BOLD" "$*" "$C_RESET"; }
info() { printf '    %s\n' "$*"; }
ok()   { printf '    %s+%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '    %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()  { printf '\n%sError:%s %s\n' "$C_RED$C_BOLD" "$C_RESET" "$*" >&2; exit 1; }

usage() {
  cat <<USAGE
Install the WSO2 Customer Data Service beside an existing Identity Server.

Usage: install_cds.sh --is-home <IS_HOME> [options]

Options:
  --is-home <path>    the Identity Server installation to enable CDS against
  --cds-home <path>   where to install CDS (default: <IS_HOME>'s parent/${PRODUCT})
  --version <vX.Y.Z>  install this CDS release instead of resolving one
  --mirror <url>      fetch the artifacts from this base URL instead of the
                      GitHub releases, laid out as <url>/<tag>/<file>
  --accept-license    accept LICENSE.txt without being asked
  --no-enable         install only; do not run cds enable
  -h, --help          this message
USAGE
}

need_value() { [ $# -ge 2 ] && [ -n "$2" ] || die "$1 requires a value"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --is-home)        need_value "$1" "${2:-}"; IS_HOME="$2"; shift 2 ;;
    --cds-home)       need_value "$1" "${2:-}"; CDS_HOME="$2"; shift 2 ;;
    --version)        need_value "$1" "${2:-}"; VERSION="$2"; shift 2 ;;
    --mirror)         need_value "$1" "${2:-}"; DOWNLOAD_BASE="${2%/}"; shift 2 ;;
    --accept-license) ACCEPT_LICENSE="1"; shift ;;
    --no-enable)      RUN_ENABLE="0"; shift ;;
    -h|--help)        usage; exit 0 ;;
    *)                usage >&2; die "unknown option $1" ;;
  esac
done

for binary in curl unzip; do
  command -v "$binary" >/dev/null 2>&1 || die "$binary is required but was not found on PATH"
done

# --------------------------------------------------------------------------- #
# 1. Read the Identity Server installation
# --------------------------------------------------------------------------- #
step "Identity Server"
[ -n "$IS_HOME" ] || { usage >&2; die "--is-home is required"; }
[ -d "$IS_HOME" ] || die "$IS_HOME does not exist"
[ -f "$IS_HOME/repository/conf/deployment.toml" ] \
  || die "$IS_HOME does not look like an Identity Server installation"

IS_HOME="$(cd "$IS_HOME" && pwd)"
IS_VERSION=""
for marker in updates/product.txt bin/version.txt repository/conf/carbon.xml; do
  [ -f "$IS_HOME/$marker" ] || continue
  IS_VERSION="$(grep -o '[0-9]\{1,\}\.[0-9]\{1,\}\.[0-9]\{1,\}' "$IS_HOME/$marker" | head -1)"
  [ -n "$IS_VERSION" ] && break
done
[ -n "$IS_VERSION" ] || die "could not determine the Identity Server version under $IS_HOME"
info "$IS_HOME"
ok "Identity Server $IS_VERSION"

# --------------------------------------------------------------------------- #
# 2. Detect the platform
# --------------------------------------------------------------------------- #
step "Platform"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)             die "unsupported CPU architecture $(uname -m); CDS is released for amd64 and arm64" ;;
esac
[ "$OS" = "linux" ] || die "unsupported operating system '$OS'; CDS is released for Linux"
ok "$OS/$ARCH"

# --------------------------------------------------------------------------- #
# 3. Resolve a compatible CDS release
#
# Every release publishes its compatibility metadata beside the zips, and that
# metadata names the Identity Server versions the release was verified against.
# Resolution walks releases newest first and takes the first that names this
# server, so the choice is the release's own statement rather than a guess made
# here.
# --------------------------------------------------------------------------- #
metadata_url() { printf '%s/%s/%s-%s-release-metadata.yaml' "$DOWNLOAD_BASE" "$1" "$PRODUCT" "${1#v}"; }

supports_is() {
  # The supported list is a YAML flow or block sequence under identity_server.
  # An empty list means the release declares no verified baseline.
  local metadata="$1"
  printf '%s\n' "$metadata" \
    | awk '/^[[:space:]]*identity_server:/{inside=1; next}
           inside && /^[[:space:]]*[a-z_]+:[[:space:]]*$/ && !/supported/{inside=0}
           inside' \
    | grep -q "$IS_VERSION"
}

step "Resolving a compatible CDS release"
if [ -n "$VERSION" ]; then
  [ "${VERSION#v}" = "$VERSION" ] && VERSION="v$VERSION"
  info "using the requested release $VERSION"
else
  TAGS="$(curl -fsSL "$API/repos/$REPO/releases?per_page=30" \
    | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | sed 's/.*"\([^"]*\)"$/\1/')" \
    || die "could not list the releases of $REPO"
  [ -n "$TAGS" ] || die "$REPO has published no releases"

  for tag in $TAGS; do
    metadata="$(curl -fsSL "$(metadata_url "$tag")" 2>/dev/null || true)"
    [ -n "$metadata" ] || continue
    if supports_is "$metadata"; then
      VERSION="$tag"
      ok "$tag declares support for Identity Server $IS_VERSION"
      break
    fi
  done

  if [ -z "$VERSION" ]; then
    VERSION="$(printf '%s\n' "$TAGS" | head -1)"
    warn "no published release declares support for Identity Server $IS_VERSION"
    warn "installing the newest release, $VERSION; cds enable will say so again"
  fi
fi

PACK_VERSION="${VERSION#v}"
PACK="$PRODUCT-$OS-$ARCH-$PACK_VERSION"
ZIP_URL="$DOWNLOAD_BASE/$VERSION/$PACK.zip"

# --------------------------------------------------------------------------- #
# 4. Choose CDS_HOME
# --------------------------------------------------------------------------- #
if [ -z "$CDS_HOME" ]; then
  CDS_HOME="$(dirname "$IS_HOME")/$PRODUCT"
fi
step "Installing to $CDS_HOME"
if [ -e "$CDS_HOME" ] && [ -n "$(ls -A "$CDS_HOME" 2>/dev/null)" ]; then
  die "$CDS_HOME exists and is not empty. Pass --cds-home <path> for a new installation,
or run $CDS_HOME/cds enable to re-enable the one that is already there"
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

info "downloading $PACK.zip"
curl -fsSL -o "$WORK/$PACK.zip" "$ZIP_URL" \
  || die "could not download $ZIP_URL
The release may not publish a pack for $OS/$ARCH."

# The release publishes a checksum beside every zip; a download that does not
# match it is not installed.
if curl -fsSL -o "$WORK/$PACK.zip.sha256" "$ZIP_URL.sha256" 2>/dev/null; then
  expected="$(awk '{print $1; exit}' "$WORK/$PACK.zip.sha256")"
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$WORK/$PACK.zip" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "$WORK/$PACK.zip" | awk '{print $1}')"
  fi
  [ "$expected" = "$actual" ] || die "the downloaded $PACK.zip does not match its published checksum"
  ok "checksum verified"
else
  warn "no published checksum for $PACK.zip, so the download could not be verified"
fi

unzip -q "$WORK/$PACK.zip" -d "$WORK/extracted" || die "could not extract $PACK.zip"

# --------------------------------------------------------------------------- #
# 5. License
# --------------------------------------------------------------------------- #
LICENSE="$WORK/extracted/$PACK/LICENSE.txt"
if [ "$ACCEPT_LICENSE" != "1" ]; then
  step "License"
  [ -f "$LICENSE" ] || die "the pack carries no LICENSE.txt"
  ${PAGER:-more} "$LICENSE" || true
  printf '\n    Accept the license and continue? [y/N] '
  read -r answer
  case "$answer" in
    y|Y|yes|YES) ;;
    *) die "the license was not accepted, so nothing was installed" ;;
  esac
fi

mkdir -p "$CDS_HOME"
# Move the pack's contents up, so CDS_HOME is the installation root rather
# than its parent.
(cd "$WORK/extracted/$PACK" && tar cf - .) | (cd "$CDS_HOME" && tar xf -)
chmod +x "$CDS_HOME/$PRODUCT"
ok "installed CDS $PACK_VERSION in $CDS_HOME"

# --------------------------------------------------------------------------- #
# 6. Hand over to the packaged enable flow
# --------------------------------------------------------------------------- #
if [ "$RUN_ENABLE" != "1" ]; then
  step "Installed"
  info "Enable CDS when you are ready:"
  info ""
  info "    $CDS_HOME/$PRODUCT enable --is-home $IS_HOME"
  exit 0
fi

step "Enabling CDS"
exec "$CDS_HOME/$PRODUCT" enable --is-home "$IS_HOME"
