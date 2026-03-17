#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Configure k3s agents to join a k3s server created by HKIC samples.

Usage:
  hack/configure-k3s-join-agents.sh <server-cr-name> [agent-label-selector] [ssh-key-path] [timeout-seconds]

Defaults:
  server-cr-name: required
  agent-label-selector: app.kubernetes.io/name=k3s-join,role=k3s-agent
  ssh-key-path: ~/.ssh/id_ed25519
  timeout-seconds: 600

Behavior:
  1) Resolves server and agent public IPs from HCloudServer status (waits until ready)
  2) Reads server node token + private IP via SSH from server
  3) Installs/reconfigures k3s-agent on each discovered agent with real server URL/token

Examples:
  hack/configure-k3s-join-agents.sh k3s-join-server
  hack/configure-k3s-join-agents.sh k3s-join-server "app.kubernetes.io/name=k3s-join,role=k3s-agent"
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 1 ]]; then
  usage
  exit 0
fi

SERVER_CR="$1"
AGENT_SELECTOR="${2:-app.kubernetes.io/name=k3s-join,role=k3s-agent}"
SSH_KEY_PATH="${3:-$HOME/.ssh/id_ed25519}"
TIMEOUT_SECONDS="${4:-600}"

if [[ ! -f "$SSH_KEY_PATH" ]]; then
  echo "SSH key not found: $SSH_KEY_PATH" >&2
  exit 1
fi

if ! [[ "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "timeout-seconds must be a positive integer, got: $TIMEOUT_SECONDS" >&2
  exit 1
fi

wait_for_public_ip() {
  local server_name="$1"
  local deadline=$((SECONDS + TIMEOUT_SECONDS))
  local ip=""
  while (( SECONDS < deadline )); do
    ip="$(kubectl get hcloudserver "$server_name" -o jsonpath='{.status.publicIPv4}' 2>/dev/null || true)"
    if [[ -n "$ip" ]]; then
      echo "$ip"
      return 0
    fi
    sleep 5
  done
  return 1
}

echo "Resolving server IP from $SERVER_CR..."
SERVER_PUBLIC_IP="$(wait_for_public_ip "$SERVER_CR")" || {
  echo "Server $SERVER_CR has no status.publicIPv4 after ${TIMEOUT_SECONDS}s." >&2
  exit 1
}

AGENT_CRS_RAW="$(kubectl get hcloudserver -l "$AGENT_SELECTOR" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
if [[ -z "$AGENT_CRS_RAW" ]]; then
  echo "No agent HCloudServer resources found with selector: $AGENT_SELECTOR" >&2
  exit 1
fi

readarray -t AGENT_CRS <<< "$AGENT_CRS_RAW"

echo "Fetching server token/private IP from $SERVER_CR ($SERVER_PUBLIC_IP)..."
SERVER_TOKEN="$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=accept-new "root@$SERVER_PUBLIC_IP" "cat /var/lib/rancher/k3s/server/node-token")"
SERVER_PRIVATE_IP="$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=accept-new "root@$SERVER_PUBLIC_IP" "ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if(\$i==\"src\"){print \$(i+1); exit}}'")"

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

  echo "Configuring $agent_cr ($agent_ip)..."
  ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=accept-new "root@$agent_ip" \
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
echo "  ssh -i \"$SSH_KEY_PATH\" root@$SERVER_PUBLIC_IP 'k3s kubectl get nodes -o wide'"
