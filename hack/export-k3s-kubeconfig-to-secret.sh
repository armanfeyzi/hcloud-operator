#!/usr/bin/env bash
# Deprecated: use contrib/k3s-optional/export-k3s-kubeconfig-to-secret.sh (this file forwards there).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$ROOT/contrib/k3s-optional/export-k3s-kubeconfig-to-secret.sh" "$@"
