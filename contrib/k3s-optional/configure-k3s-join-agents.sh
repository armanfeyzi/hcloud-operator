#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'EOF'
Configure k3s agents to join a k3s server created by HKIC samples.

Usage:
  contrib/k3s-optional/configure-k3s-join-agents.sh <server-cr-name> [agent-label-selector] [ssh-key-path] [timeout-seconds]

Defaults:
  server-cr-name: required
  agent-label-selector: auto-derived from server label app.kubernetes.io/name, plus role=k3s-agent
  ssh-key-path: ~/.ssh/id_ed25519
  timeout-seconds: 600

Behavior:
  1) Validates spec.sshKeys on server + agents (fails fast on empty/REPLACE_* before any SSH wait)
  2) Resolves server public IP, waits for running + SSH
  3) Reads server node token + private IP via SSH from server
  4) Installs/reconfigures k3s-agent on each discovered agent with real server URL/token

Environment:
  HKC_KUBECONFIG   If KUBECONFIG points at a workload kubeconfig (e.g. ./k3s-hc.yaml), set this to the
                   kubeconfig path for the cluster that hosts HCloudServer CRs (e.g. "$HOME/.kube/config").

Examples (from repo root):
  ./contrib/k3s-optional/configure-k3s-join-agents.sh k3s-join-server
  ./contrib/k3s-optional/configure-k3s-join-agents.sh k3s-shape-server "" ~/.ssh/id_ed25519
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 1 ]]; then
  usage
  exit 0
fi

SERVER_CR="$1"
AGENT_SELECTOR="${2:-}"
SSH_KEY_PATH="${3:-$HOME/.ssh/id_ed25519}"
TIMEOUT_SECONDS="${4:-600}"

if ! [[ "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "timeout-seconds must be a positive integer, got: $TIMEOUT_SECONDS" >&2
  exit 1
fi

# shellcheck source=k3s-remote-common.inc.sh
source "$SCRIPT_DIR/k3s-remote-common.inc.sh"

if [[ -z "$AGENT_SELECTOR" ]]; then
  server_app_name="$(hkc_kubectl get hcloudserver "$SERVER_CR" -o jsonpath='{.metadata.labels.app\.kubernetes\.io/name}' 2>/dev/null || true)"
  if [[ -n "$server_app_name" ]]; then
    AGENT_SELECTOR="app.kubernetes.io/name=${server_app_name},role=k3s-agent"
  else
    AGENT_SELECTOR="role=k3s-agent"
  fi
fi

AGENT_CRS_RAW="$(hkc_kubectl get hcloudserver -l "$AGENT_SELECTOR" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
if [[ -z "$AGENT_CRS_RAW" ]]; then
  echo "No agent HCloudServer resources found with selector: $AGENT_SELECTOR" >&2
  exit 1
fi

readarray -t AGENT_CRS <<< "$AGENT_CRS_RAW"
FILTERED_AGENT_CRS=()
for agent_cr in "${AGENT_CRS[@]}"; do
  if [[ -n "$agent_cr" && "$agent_cr" != "$SERVER_CR" ]]; then
    FILTERED_AGENT_CRS+=("$agent_cr")
  fi
done
AGENT_CRS=("${FILTERED_AGENT_CRS[@]}")
if [[ "${#AGENT_CRS[@]}" -eq 0 ]]; then
  echo "Selector matched only server $SERVER_CR; no agent nodes found." >&2
  exit 1
fi

for _ssh_cr in "$SERVER_CR" "${AGENT_CRS[@]}"; do
  assert_hcloudserver_ssh_keys_look_sane "$_ssh_cr" || exit 1
done

echo "Resolving server IP from $SERVER_CR..."
SERVER_PUBLIC_IP="$(wait_for_public_ip "$SERVER_CR")" || {
  echo "Server $SERVER_CR has no status.publicIPv4 after ${TIMEOUT_SECONDS}s." >&2
  exit 1
}

wait_for_hcloudserver_running "$SERVER_CR" || exit 1
wait_for_ssh_ready "$SERVER_PUBLIC_IP" "$SERVER_CR" "$SERVER_CR" || exit 1

echo "Fetching server token/private IP from $SERVER_CR ($SERVER_PUBLIC_IP)..."
SERVER_TOKEN="$(ssh "${SSH_BASE_OPTS[@]}" "root@$SERVER_PUBLIC_IP" "cat /var/lib/rancher/k3s/server/node-token")"
SERVER_PRIVATE_IP="$(ssh "${SSH_BASE_OPTS[@]}" "root@$SERVER_PUBLIC_IP" "ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if(\$i==\"src\"){print \$(i+1); exit}}'")"

if [[ -z "$SERVER_TOKEN" || -z "$SERVER_PRIVATE_IP" ]]; then
  echo "Could not resolve server token/private IP." >&2
  exit 1
fi

SERVER_URL="https://${SERVER_PRIVATE_IP}:6443"
echo "Using server URL: $SERVER_URL"
echo "Discovered agents: ${AGENT_CRS[*]}"

for agent_cr in "${AGENT_CRS[@]}"; do
  agent_ip="$(wait_for_public_ip "$agent_cr")" || {
    echo "Skipping $agent_cr: no status.publicIPv4 after ${TIMEOUT_SECONDS}s"
    continue
  }

  wait_for_hcloudserver_running "$agent_cr" || {
    echo "Skipping $agent_cr: did not reach running in time"
    continue
  }

  wait_for_ssh_ready "$agent_ip" "$agent_cr" "$agent_cr" || {
    echo "Skipping $agent_cr: SSH not available in time"
    continue
  }

  echo "Configuring $agent_cr ($agent_ip)..."
  ssh "${SSH_BASE_OPTS[@]}" "root@$agent_ip" \
    "set -euo pipefail; \
     PRIVIP=\$(ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if(\$i==\"src\"){print \$(i+1); exit}}'); \
     PRIVIF=\$(ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if(\$i==\"dev\"){print \$(i+1); exit}}'); \
     mkdir -p /etc/rancher/k3s; \
     cat > /etc/rancher/k3s/config.yaml <<CFG
server: ${SERVER_URL}
token: ${SERVER_TOKEN}
\${PRIVIP:+node-ip: \${PRIVIP}}
\${PRIVIF:+flannel-iface: \${PRIVIF}}
CFG
     if command -v k3s-agent >/dev/null 2>&1; then
       systemctl restart k3s-agent
     else
       curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='agent' sh -
     fi"
done

echo "Done. Verify on server with:"
echo "  ssh -i \"$SSH_KEY_PATH\" -o StrictHostKeyChecking=accept-new -o BatchMode=yes root@$SERVER_PUBLIC_IP 'k3s kubectl get nodes -o wide'"
