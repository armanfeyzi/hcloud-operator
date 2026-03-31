#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'EOF'
Verify a multi-node k3s cluster created by HKIC sample manifests.

Usage:
  contrib/k3s-optional/verify-k3s-join-cluster.sh <server-cr-name> [expected-node-count] [ssh-key-path] [timeout-seconds] [check-day2]

Defaults:
  server-cr-name: required
  expected-node-count: 3
  ssh-key-path: ~/.ssh/id_ed25519
  timeout-seconds: 600
  check-day2: false (set true to run CSI + CCM smoke checks)

Behavior:
  1) Resolves server public IP from HCloudServer status (waits until ready)
  2) Waits for status.state=running, then SSH readiness (TCP :22 + public key auth)
  3) Runs health checks from the server using `k3s kubectl`
  4) Waits for expected node count and Ready status, then checks core system pods
  5) Optional: runs Day-2 smoke checks (StorageClass + CSI PVC/Pod + CCM LoadBalancer)

Examples (from repo root):
  ./contrib/k3s-optional/verify-k3s-join-cluster.sh k3s-join-server
  ./contrib/k3s-optional/verify-k3s-join-cluster.sh k3s-join-server 3 ~/.ssh/id_ed25519 600 true
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 1 ]]; then
  usage
  exit 0
fi

SERVER_CR="$1"
EXPECTED_NODES="${2:-3}"
SSH_KEY_PATH="${3:-$HOME/.ssh/id_ed25519}"
TIMEOUT_SECONDS="${4:-600}"
CHECK_DAY2="${5:-false}"

if [[ ! -f "$SSH_KEY_PATH" ]]; then
  echo "SSH key not found: $SSH_KEY_PATH" >&2
  exit 1
fi

if ! [[ "$EXPECTED_NODES" =~ ^[0-9]+$ ]] || [[ "$EXPECTED_NODES" -lt 1 ]]; then
  echo "expected-node-count must be an integer >= 1, got: $EXPECTED_NODES" >&2
  exit 1
fi

if ! [[ "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT_SECONDS" -lt 1 ]]; then
  echo "timeout-seconds must be an integer >= 1, got: $TIMEOUT_SECONDS" >&2
  exit 1
fi

if [[ "$CHECK_DAY2" != "true" && "$CHECK_DAY2" != "false" ]]; then
  echo "check-day2 must be either true or false, got: $CHECK_DAY2" >&2
  exit 1
fi

# shellcheck source=k3s-remote-common.inc.sh
source "$SCRIPT_DIR/k3s-remote-common.inc.sh"

assert_hcloudserver_ssh_keys_look_sane "$SERVER_CR" || exit 1

echo "Resolving server IP from $SERVER_CR..."
SERVER_PUBLIC_IP="$(wait_for_public_ip "$SERVER_CR")" || {
  echo "Server $SERVER_CR has no status.publicIPv4 after ${TIMEOUT_SECONDS}s." >&2
  exit 1
}

echo "Using server: $SERVER_CR ($SERVER_PUBLIC_IP)"

wait_for_hcloudserver_running "$SERVER_CR" || exit 1
wait_for_ssh_ready "$SERVER_PUBLIC_IP" "$SERVER_CR" "$SERVER_CR" || exit 1

remote() {
  ssh "${SSH_BASE_OPTS[@]}" "root@$SERVER_PUBLIC_IP" "$@"
}

echo "Checking k3s control plane response..."
remote "k3s kubectl version --short"

echo "Waiting for ${EXPECTED_NODES} node(s) to register..."
deadline=$((SECONDS + TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  node_count="$(remote "k3s kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' '" || true)"
  if [[ "$node_count" -ge "$EXPECTED_NODES" ]]; then
    break
  fi
  sleep 5
done

node_count="$(remote "k3s kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' '" || true)"
if [[ "$node_count" -lt "$EXPECTED_NODES" ]]; then
  echo "Expected at least $EXPECTED_NODES nodes, found $node_count." >&2
  remote "k3s kubectl get nodes -o wide || true"
  exit 1
fi

echo "Waiting for all nodes to become Ready..."
remote "k3s kubectl wait --for=condition=Ready node --all --timeout=${TIMEOUT_SECONDS}s"

echo "Checking core system pods..."
remote "k3s kubectl get pods -n kube-system"
remote "k3s kubectl wait --for=condition=Ready pod --all -n kube-system --timeout=${TIMEOUT_SECONDS}s"

echo "Cluster looks healthy."
remote "k3s kubectl get nodes -o wide"

if [[ "$CHECK_DAY2" == "true" ]]; then
  echo "Running Day-2 smoke checks (CSI + CCM)..."
  ssh "${SSH_BASE_OPTS[@]}" "root@$SERVER_PUBLIC_IP" \
    "TIMEOUT_SECONDS=$TIMEOUT_SECONDS bash -s" <<'EOF'
set -euo pipefail

echo "Checking StorageClass hcloud-volumes..."
k3s kubectl get storageclass hcloud-volumes >/dev/null

echo "Running CSI smoke resources..."
cat <<'YAML' | k3s kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-smoke-pvc
  namespace: default
spec:
  storageClassName: hcloud-volumes
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: csi-smoke-pod
  namespace: default
spec:
  containers:
    - name: writer
      image: busybox:1.36
      command: ["/bin/sh", "-c", "echo csi-ok > /data/ok.txt && sleep 120"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: csi-smoke-pvc
YAML

k3s kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/csi-smoke-pvc -n default --timeout="${TIMEOUT_SECONDS}s"
k3s kubectl wait --for=condition=Ready pod/csi-smoke-pod -n default --timeout="${TIMEOUT_SECONDS}s"
k3s kubectl delete pod/csi-smoke-pod pvc/csi-smoke-pvc -n default --ignore-not-found

echo "Running CCM LoadBalancer smoke resources..."
cat <<'YAML' | k3s kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ccm-lb-smoke
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ccm-lb-smoke
  template:
    metadata:
      labels:
        app: ccm-lb-smoke
    spec:
      containers:
        - name: echo
          image: hashicorp/http-echo:1.0
          args:
            - "-text=ccm-ok"
          ports:
            - containerPort: 5678
---
apiVersion: v1
kind: Service
metadata:
  name: ccm-lb-smoke
  namespace: default
spec:
  selector:
    app: ccm-lb-smoke
  ports:
    - port: 80
      targetPort: 5678
  type: LoadBalancer
YAML

k3s kubectl wait --for=condition=Available deployment/ccm-lb-smoke -n default --timeout="${TIMEOUT_SECONDS}s"

deadline=$((SECONDS + TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  lb_ip="$(k3s kubectl get svc ccm-lb-smoke -n default -o jsonpath='{.status.loadBalancer.ingress[0].ip}' || true)"
  lb_hostname="$(k3s kubectl get svc ccm-lb-smoke -n default -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' || true)"
  if [[ -n "$lb_ip" || -n "$lb_hostname" ]]; then
    break
  fi
  sleep 5
done

lb_ip="$(k3s kubectl get svc ccm-lb-smoke -n default -o jsonpath='{.status.loadBalancer.ingress[0].ip}' || true)"
lb_hostname="$(k3s kubectl get svc ccm-lb-smoke -n default -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' || true)"
if [[ -z "$lb_ip" && -z "$lb_hostname" ]]; then
  echo "CCM smoke service did not receive external ingress within timeout." >&2
  k3s kubectl get svc ccm-lb-smoke -n default -o wide || true
  exit 1
fi

k3s kubectl delete svc/ccm-lb-smoke deployment/ccm-lb-smoke -n default --ignore-not-found
echo "Day-2 smoke checks passed."
EOF
fi
