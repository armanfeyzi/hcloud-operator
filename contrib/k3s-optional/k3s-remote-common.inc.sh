# Shared helpers for contrib/k3s-optional/configure-k3s-join-agents.sh and contrib/k3s-optional/verify-k3s-join-cluster.sh
# shellcheck shell=bash

# kubectl calls in this file target the cluster that hosts HCloudServer CRs (HKIC / kind).
# If your shell has KUBECONFIG pointed at a workload file (e.g. ./k3s-hc.yaml), set:
#   export HKC_KUBECONFIG="$HOME/.kube/config"
hkc_kubectl() {
  if [[ -n "${HKC_KUBECONFIG:-}" ]]; then
    command kubectl --kubeconfig "$HKC_KUBECONFIG" "$@"
  else
    command kubectl "$@"
  fi
}

: "${SSH_KEY_PATH:?}"
: "${TIMEOUT_SECONDS:?}"

if [[ ! -f "$SSH_KEY_PATH" ]]; then
  echo "SSH key not found: $SSH_KEY_PATH" >&2
  exit 1
fi

# -4: avoid slow/failed IPv6 paths when target is IPv4
# IdentitiesOnly: do not walk the ssh-agent offering unrelated keys (can feel "stuck")
SSH_BASE_OPTS=(
  -4
  -i "$SSH_KEY_PATH"
  -o IdentitiesOnly=yes
  -o StrictHostKeyChecking=accept-new
  -o BatchMode=yes
  -o LogLevel=ERROR
  -o ConnectTimeout=10
  -o ConnectionAttempts=1
  -o PreferredAuthentications=publickey
  -o PubkeyAuthentication=yes
  -o NumberOfPasswordPrompts=0
)

# Fail fast before long waits: Hetzner only injects keys at server *create* time.
assert_hcloudserver_ssh_keys_look_sane() {
  local cr="$1"
  local keys=""
  keys="$(hkc_kubectl get hcloudserver "$cr" -o jsonpath='{.spec.sshKeys[*]}' 2>/dev/null || true)"
  if [[ -z "${keys// }" ]]; then
    cat >&2 <<EOF2
HCloudServer ${cr} has empty spec.sshKeys.
Hetzner will not install your public key; root login will fail with Permission denied.
Set spec.sshKeys to the SSH key *name* as shown in Hetzner Cloud (Security → SSH keys), then delete/recreate the server.
EOF2
    return 1
  fi
  if [[ "$keys" == *REPLACE_* ]]; then
    cat >&2 <<EOF2
HCloudServer ${cr} still uses a placeholder in spec.sshKeys (${keys}).
This is whatever the cluster object currently has (kubectl get), not your shell arguments: fix every HCloudServer in your manifest,
then delete and recreate those servers so Hetzner applies keys at create time.

Check cluster: kubectl get hcloudserver ${cr} -o jsonpath='{.spec.sshKeys}{"\n"}'
EOF2
    return 1
  fi
  return 0
}

fatal_ssh_auth_mismatch_help() {
  local host_ip="$1"
  local cr_name="${2:-}"
  cat >&2 <<EOF2
SSH authentication failed repeatedly to ${host_ip} while port 22 is open.
This is not a boot-timing issue: the server does not trust this private key (${SSH_KEY_PATH}).

Fix (pick what applies):
  1) Ensure HCloudServer spec.sshKeys lists the exact key *label* from Hetzner Cloud (not the public key string).
  2) Ensure the private key you pass to this script matches that Hetzner key (try: ssh -i "${SSH_KEY_PATH}" -o StrictHostKeyChecking=accept-new -o BatchMode=yes root@${host_ip} true )
  3) If you changed sshKeys after the VM existed, you must delete and recreate the HCloudServer (Hetzner does not retrofit keys).

Inspect: kubectl get hcloudserver ${cr_name:-<name>} -o yaml | sed -n '/sshKeys:/,/^[a-z]/p'
EOF2
}

tcp_port_open() {
  local host="$1"
  local port="$2"
  if command -v nc >/dev/null 2>&1; then
    nc -z -w3 "$host" "$port" >/dev/null 2>&1
    return $?
  fi
  # bash built-in TCP (no nc required)
  (echo >/dev/tcp/"${host}"/"${port}") >/dev/null 2>&1
}

wait_for_public_ip() {
  local server_name="$1"
  local deadline=$((SECONDS + TIMEOUT_SECONDS))
  local ip=""
  while (( SECONDS < deadline )); do
    ip="$(hkc_kubectl get hcloudserver "$server_name" -o jsonpath='{.status.publicIPv4}' 2>/dev/null || true)"
    if [[ -n "$ip" ]]; then
      echo "$ip"
      return 0
    fi
    sleep 5
  done
  return 1
}

# Prefer Hetzner "running" before spending time on SSH (IP can exist while still off/initializing).
wait_for_hcloudserver_running() {
  local server_name="$1"
  local deadline=$((SECONDS + TIMEOUT_SECONDS))
  local state=""
  local last_progress="$SECONDS"
  echo "Waiting for $server_name status.state=running..."
  while (( SECONDS < deadline )); do
    state="$(hkc_kubectl get hcloudserver "$server_name" -o jsonpath='{.status.state}' 2>/dev/null || true)"
    if [[ "$state" == "running" ]]; then
      return 0
    fi
    if (( SECONDS - last_progress >= 30 )); then
      echo "  ... $server_name state=${state:-unknown}" >&2
      last_progress=$SECONDS
    fi
    sleep 5
  done
  echo "Timed out waiting for $server_name to reach running (last state: ${state:-unknown})." >&2
  return 1
}

# Wait until TCP/22 accepts and SSH public-key auth works. Prints progress and last SSH error.
wait_for_ssh_ready() {
  local host_ip="$1"
  local label="$2"
  local cr_name="${3:-}"
  local deadline=$((SECONDS + TIMEOUT_SECONDS))
  local last_progress="$SECONDS"
  local ssh_err=""
  local ssh_out=""
  local auth_fail_streak=0

  echo "Waiting for SSH on $label ($host_ip) (timeout ${TIMEOUT_SECONDS}s)..."
  while (( SECONDS < deadline )); do
    local hint=""
    if [[ -n "$cr_name" ]]; then
      hint="$(hkc_kubectl get hcloudserver "$cr_name" -o jsonpath='{.metadata.name}: state={.status.state} ipv4={.status.publicIPv4}' 2>/dev/null || true)"
    fi

    if tcp_port_open "$host_ip" 22; then
      ssh_out="$(ssh "${SSH_BASE_OPTS[@]}" "root@${host_ip}" "true" 2>&1)" && {
        echo "SSH ready on $label ($host_ip)."
        return 0
      }
      ssh_err="$ssh_out"
      # Port open + permission denied will not heal by waiting; fail fast.
      if echo "$ssh_out" | grep -qi 'permission denied'; then
        ((++auth_fail_streak)) || true
        if (( auth_fail_streak >= 3 )); then
          fatal_ssh_auth_mismatch_help "$host_ip" "$cr_name"
          return 1
        fi
      else
        auth_fail_streak=0
      fi
      echo "Port 22 is open on $host_ip but SSH failed (will retry): ${ssh_err:-unknown}" >&2
    else
      auth_fail_streak=0
    fi

    if (( SECONDS - last_progress >= 20 )); then
      echo "  ... still waiting (${hint:-no kubectl hint})" >&2
      last_progress=$SECONDS
    fi
    sleep 3
  done

  echo "Timed out waiting for SSH on $host_ip (${label})." >&2
  echo "Last SSH attempt output: ${ssh_err:-n/a}" >&2
  fatal_ssh_auth_mismatch_help "$host_ip" "$cr_name"
  return 1
}
