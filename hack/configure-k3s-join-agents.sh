#!/usr/bin/env bash
# Deprecated: use contrib/k3s-optional/configure-k3s-join-agents.sh (this file forwards there).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$ROOT/contrib/k3s-optional/configure-k3s-join-agents.sh" "$@"
