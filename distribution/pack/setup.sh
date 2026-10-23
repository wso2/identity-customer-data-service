#!/usr/bin/env bash
#
# Establishes, checks or removes this installation's relationship with a WSO2
# Identity Server.
#
# A thin entry point over cds, kept at the pack root so that the shape
# matches the other on-premises products. It holds no logic of its own: every
# option below forwards to the command of the same name, and anything else on
# the line is passed through untouched.
#
#   ./setup.sh --is-home /opt/wso2is     enable CDS against that Identity Server
#   ./setup.sh --disable                 take the CDS integration back out
#   ./setup.sh --verify                  check the integration, changing nothing
#   ./setup.sh --help                    the options of the underlying command

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CDS="$HERE/cds"

[ -x "$CDS" ] || { echo "setup: $CDS is missing or not executable" >&2; exit 1; }

usage() {
  cat <<USAGE
Set up CDS with a WSO2 Identity Server.

Usage: ./setup.sh [--disable|--verify] [options]

  (no action)   enable CDS against an Identity Server
  --disable     remove the CDS integration from the associated Identity Server
  --verify      check CDS and its integration, changing nothing
  --help        show the options of the resolved command

Options other than the action are passed through, so:

  ./setup.sh --is-home /opt/wso2is
  ./setup.sh --is-home /opt/wso2is --extensions /media/bundles
USAGE
}

# The action is taken out of the arguments; everything else goes through in
# the order it was given.
COMMAND="enable"
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --disable) COMMAND="disable" ;;
    --verify)  COMMAND="verify" ;;
    -h)        usage; exit 0 ;;
    *)         ARGS+=("$arg") ;;
  esac
done

exec "$CDS" "$COMMAND" ${ARGS+"${ARGS[@]}"}
