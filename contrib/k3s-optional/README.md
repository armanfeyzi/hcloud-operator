# Optional k3s helpers (not part of the operator)

These scripts support **advanced** [k3s on Hetzner](../../docs/k3s-on-hcloud.md) samples: SSH bootstrap, agent join, kubeconfig export, and optional smoke checks.

They are **not** required to install or run HKIC, and they are **not** reconciled by the operator. Treat them like cookbook utilities next to the CRDs.

| Script | Purpose |
|--------|---------|
| `configure-k3s-join-agents.sh` | After multi-node samples are applied, SSH to server/agents and configure k3s agent join. |
| `verify-k3s-join-cluster.sh` | SSH to the control-plane node and run basic (optional Day-2) health checks. |
| `export-k3s-kubeconfig-to-secret.sh` | Copy `k3s.yaml` off the server and store it as a Kubernetes Secret. |
| `k3s-remote-common.inc.sh` | Shared bash helpers (sourced by the scripts above; do not run directly). |

From the repository root:

```bash
chmod +x contrib/k3s-optional/*.sh
./contrib/k3s-optional/configure-k3s-join-agents.sh --help
```

For GitOps-focused workflows, prefer declaring **`HCloudServer` / `HCloudNetwork` / …** in your repo; use these scripts only when you explicitly want the SSH-based join path from the docs.
