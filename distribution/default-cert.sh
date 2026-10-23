#!/usr/bin/env bash
#
# Generates the default TLS key pair a pack ships with.
#
# CDS serves HTTPS only and will not start without a certificate, and the
# Identity Server has to verify it, so a pack that carries none cannot be
# started or enabled in one pass. WSO2 Identity Server solves this the same
# way: it ships a self-signed CN=localhost key pair in every download.
#
# Two deliberate differences from simply committing one:
#
#   * It is generated per build rather than kept in the repository, so no
#     private key is ever committed to a public git history.
#   * The subject marks it as a default, so the runtime can recognise it and
#     say that it is still in use.
#
# This is a convenience for getting started, NOT a production credential:
# every download of a release carries the same key, so anyone holding it can
# present themselves as that CDS until an operator replaces it.
#
#   default-cert.sh <security-dir>

set -euo pipefail

DIR="${1:?usage: default-cert.sh <security-dir>}"
mkdir -p "$DIR"

command -v openssl >/dev/null 2>&1 || {
  echo "default-cert: openssl is required to generate the default certificate" >&2
  exit 1
}

# OU is the marker the runtime matches on. Changing it means changing
# defaultCertificateOU in internal/cli as well.
SUBJECT="/C=US/ST=CA/L=Santa Clara/O=WSO2/OU=Customer Data Service Default/CN=localhost"

openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
  -subj "$SUBJECT" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
  -addext "keyUsage=critical,digitalSignature,keyEncipherment" \
  -addext "extendedKeyUsage=serverAuth" \
  -keyout "$DIR/server.key" \
  -out "$DIR/server.crt" >/dev/null 2>&1 || {
    echo "default-cert: openssl could not generate the key pair" >&2
    exit 1
  }

# The key is readable by its owner alone, even inside the zip, so an extracted
# pack does not start out world-readable.
chmod 600 "$DIR/server.key"
chmod 644 "$DIR/server.crt"
