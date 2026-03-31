#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 2 ]]; then
  cat <<'EOF'
Export a remote k3s kubeconfig and store it as a Kubernetes Secret.

Usage:
  contrib/k3s-optional/export-k3s-kubeconfig-to-secret.sh <server-public-ip> <secret-name> [namespace] [ssh-key-path]

Arguments:
  server-public-ip   Public IP of the k3s server node.
  secret-name        Name of Secret to create/update.
  namespace          Namespace for Secret (default: default).
  ssh-key-path       SSH private key path (default: ~/.ssh/id_ed25519).

Behavior:
  1) Copies /etc/rancher/k3s/k3s.yaml from the server via SCP
  2) Rewrites server URL from 127.0.0.1 to the provided public IP
  3) Creates/updates a Secret with key "config"

Example (from repo root):
  ./contrib/k3s-optional/export-k3s-kubeconfig-to-secret.sh 91.107.142.178 k3s-join-kubeconfig kube-public ~/.ssh/id_ed25519
EOF
  exit 0
fi

SERVER_IP="$1"
SECRET_NAME="$2"
NAMESPACE="${3:-default}"
SSH_KEY_PATH="${4:-$HOME/.ssh/id_ed25519}"

if [[ ! -f "$SSH_KEY_PATH" ]]; then
  echo "SSH key not found: $SSH_KEY_PATH" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
TMP_CONFIG="$TMP_DIR/k3s.yaml"

echo "Copying kubeconfig from root@${SERVER_IP}..."
scp -i "$SSH_KEY_PATH" "root@${SERVER_IP}:/etc/rancher/k3s/k3s.yaml" "$TMP_CONFIG"

echo "Rewriting kubeconfig server endpoint..."
sed -i "s|https://127.0.0.1:6443|https://${SERVER_IP}:6443|g" "$TMP_CONFIG"

echo "Creating/updating Secret ${NAMESPACE}/${SECRET_NAME}..."
kubectl create secret generic "$SECRET_NAME" \
  -n "$NAMESPACE" \
  --from-file=config="$TMP_CONFIG" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Done."
echo "Test with:"
echo "  kubectl get secret -n ${NAMESPACE} ${SECRET_NAME}"
echo "  kubectl get secret -n ${NAMESPACE} ${SECRET_NAME} -o jsonpath='{.data.config}' | base64 -d > ./k3s-from-secret.yaml"
echo "  KUBECONFIG=./k3s-from-secret.yaml kubectl get nodes"
