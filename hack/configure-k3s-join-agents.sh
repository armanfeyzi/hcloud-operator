#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 1 ]]; then
  cat <<'EOF'
Configure k3s agents to join a k3s server created by HKIC samples.

Usage:
  hack/configure-k3s-join-agents.sh <server-cr-name> [agent-cr-names-comma-separated] [ssh-key-path]

Defaults:
  server-cr-name: required
  agent-cr-names-comma-separated: k3s-join-agent-1,k3s-join-agent-2
  ssh-key-path: ~/.ssh/id_ed25519

Behavior:
  1) Resolves public IPs for server and agents from HCloudServer status
  2) Reads server node token + private IP via SSH from server
  3) Installs/reconfigures k3s-agent on each agent with real server URL/token

Example:
  hack/configure-k3s-join-agents.sh k3s-join-server
EOF
  exit 0
fi

SERVER_CR="$1"
AGENT_CRS_CSV="${2:-k3s-join-agent-1,k3s-join-agent-2}"
SSH_KEY_PATH="${3:-$HOME/.ssh/id_ed25519}"

if [[ ! -f "$SSH_KEY_PATH" ]]; then
  echo "SSH key not found: $SSH_KEY_PATH" >&2
  exit 1
fi

SERVER_PUBLIC_IP="$(kubectl get hcloudserver "$SERVER_CR" -o jsonpath='{.status.publicIPv4}')"
if [[ -z "$SERVER_PUBLIC_IP" ]]; then
  echo "Server $SERVER_CR has no status.publicIPv4 yet." >&2
  exit 1
fi

echo "Fetching server token/private IP from $SERVER_CR ($SERVER_PUBLIC_IP)..."
SERVER_TOKEN="$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=accept-new "root@$SERVER_PUBLIC_IP" "cat /var/lib/rancher/k3s/server/node-token")"
SERVER_PRIVATE_IP="$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=accept-new "root@$SERVER_PUBLIC_IP" "ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if(\$i==\"src\"){print \$(i+1); exit}}'")"

if [[ -z "$SERVER_TOKEN" || -z "$SERVER_PRIVATE_IP" ]]; then
  echo "Could not resolve server token/private IP." >&2
  exit 1
fi

SERVER_URL="https://${SERVER_PRIVATE_IP}:6443"
echo "Using server URL: $SERVER_URL"

IFS=',' read -r -a AGENT_CRS <<< "$AGENT_CRS_CSV"

for agent_cr in "${AGENT_CRS[@]}"; do
  agent_ip="$(kubectl get hcloudserver "$agent_cr" -o jsonpath='{.status.publicIPv4}')"
  if [[ -z "$agent_ip" ]]; then
    echo "Skipping $agent_cr: no status.publicIPv4 yet"
    continue
  fi

  echo "Configuring $agent_cr ($agent_ip)..."
  ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=accept-new "root@$agent_ip" \
    "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='agent --server=${SERVER_URL} --token=${SERVER_TOKEN}' sh -"
done

echo "Done. Verify with:"
echo "  kubectl get nodes -o wide"
