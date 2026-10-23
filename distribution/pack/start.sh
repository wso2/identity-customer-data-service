#!/usr/bin/env bash
#
# Runs the CDS service, or acts on one that is already running.
#
# A thin entry point over bin/wso2cds, kept at the pack root so that the shape
# matches the other on-premises products. It holds no logic of its own: every
# option below forwards to the command of the same name, and anything else on
# the line is passed through untouched.
#
# Running it and acting on it are separate invocations:
#
#   ./start.sh                run in the foreground; Ctrl+C stops it
#   ./start.sh --daemon       run in the background and return once ready
#   ./start.sh --stop         stop a service started with --daemon
#   ./start.sh --status       product, platform, runtime and integration state
#   ./start.sh --logs         show the log a --daemon run writes
#
# --daemon is what makes --stop and --status meaningful: a foreground run
# belongs to the terminal that started it and ends with Ctrl+C there.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CDS="$HERE/bin/wso2cds"

[ -x "$CDS" ] || { echo "start: $CDS is missing or not executable" >&2; exit 1; }

usage() {
  cat <<USAGE
Run the WSO2 Customer Data Service.

Usage: ./start.sh [--daemon]
       ./start.sh [--stop|--status|--logs] [options]

Running it:

  (no action)   run in the foreground; Ctrl+C stops it
  --daemon      run in the background and return once it reports ready

Acting on a service started with --daemon:

  --stop        stop it
  --status      show product, platform, runtime and integration state
  --logs        show the log it writes; --logs -f follows it

  --help        show the options of the resolved command

There is no --restart: it is --stop followed by a start, and writing both
says which state you expect in between.

A foreground run has nothing to --stop: it belongs to the terminal that
started it. Use --daemon when the shell has to come back.

Options other than the action are passed through, so:

  ./start.sh --daemon --timeout 90s
  ./start.sh --logs -f
USAGE
}

COMMAND="start"
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --stop)    COMMAND="stop" ;;
    --status)  COMMAND="status" ;;
    --logs)    COMMAND="logs" ;;
    -h)        usage; exit 0 ;;
    *)         ARGS+=("$arg") ;;
  esac
done

exec "$CDS" "$COMMAND" ${ARGS+"${ARGS[@]}"}
