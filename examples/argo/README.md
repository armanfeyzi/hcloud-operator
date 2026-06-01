# Argo CD — HKIC operator + demo stack

GitOps example for [Milestone 2](https://github.com/armanfeyzi/hcloud-operator/milestone/2): install the operator with Helm via Argo CD, then sync a wave-ordered Hetzner stack (network → server → volume → load balancer).

## Prerequisites

- A **management** Kubernetes cluster with [Argo CD](https://argo-cd.readthedocs.io/) installed (`argocd` CLI optional).
- A Hetzner Cloud API token with permissions to create servers, networks, volumes, and load balancers.
- At least one **SSH key** registered in the Hetzner console (for `HCloudServer.spec.sshKeys` if you enable SSH on the demo server).
- Fork or use this repository URL in the Application manifests (default: `https://github.com/armanfeyzi/hcloud-operator.git`).

## Bootstrap (one-time)

### 1. Hetzner token Secret (not in Git)

```bash
kubectl create namespace hcloud-operator-system --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic hcloud-operator-secret \
  -n hcloud-operator-system \
  --from-literal=token="$HCLOUD_TOKEN"
```

The Deployment expects Secret key **`token`** (maps to `HCLOUD_TOKEN` in the pod).

### 2. AppProject

```bash
kubectl apply -f examples/argo/project.yaml
```

Tighten `sourceRepos` in `project.yaml` for production (remove the `'*'` entry).

### 3. Operator Application (Helm chart, sync wave -1)

```bash
kubectl apply -f examples/argo/applications/operator.yaml
```

Wait until the operator is healthy:

```bash
kubectl wait --for=condition=Available deployment/hcloud-operator \
  -n hcloud-operator-system --timeout=180s
kubectl api-resources | grep infra.hkc.io
```

### 4. Edit demo manifests

Before syncing infrastructure, set your SSH key name in `examples/argo/manifests/server.yaml` (uncomment `sshKeys`) and adjust `serverType`, `location`, and `image` if needed.

Optional edge firewall: add `firewall-optional.yaml` to `manifests/kustomization.yaml` `resources`.

### 5. Infrastructure Application (waves 0–3)

```bash
kubectl apply -f examples/argo/applications/infrastructure.yaml
```

Or sync from the Argo CD UI: **hcloud-gitops-demo** → Sync.

## Sync wave order

| Wave | Resources |
|---|---|
| -1 | Operator Deployment, RBAC, CRDs (Helm `crds/`) |
| 0 | `HCloudNetwork` (+ optional `HCloudFirewall`) |
| 1 | `HCloudServer` |
| 2 | `HCloudVolume` |
| 3 | `HCloudLoadBalancer` |

Annotations: `argocd.argoproj.io/sync-wave` on each manifest metadata.

## Verification checklist

- [ ] `kubectl get application -n argocd` — `hcloud-operator` and `hcloud-gitops-demo` **Synced** / **Healthy**
- [ ] `kubectl get pods -n hcloud-operator-system` — operator **Running**
- [ ] `kubectl get hcloudserver gitops-demo-web` — `status.state=running`, public IP populated (may take several minutes)
- [ ] `kubectl get hcloudloadbalancer gitops-demo-lb` — status shows Hetzner LB ID and IPv4

## Teardown

Delete the infrastructure Application first (or delete HKIC CRs), then the operator:

```bash
kubectl delete -f examples/argo/applications/infrastructure.yaml
kubectl delete -f examples/argo/applications/operator.yaml
```

Deleting CRs triggers Hetzner cleanup via finalizers when the operator is still running.

Argo CD deletes resources in **reverse sync-wave order** (higher waves first). That means wave-2 objects (volume, firewall, floating IP, primary IP) are removed while the server (wave 1) may still exist. The operator detaches firewalls and unassigns IPs from live Hetzner state before delete so teardown does not require the server CR to disappear first.

## Related

- Helm chart: [charts/hcloud-operator/README.md](../../charts/hcloud-operator/README.md)
- Sprint plan: [docs/sprint-platform-gitops.md](../../docs/sprint-platform-gitops.md)
- Static stack sample: [config/samples/complex/hcloud-stack/](../../config/samples/complex/hcloud-stack/)
