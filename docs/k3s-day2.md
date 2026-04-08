# k3s Day-2 on Hetzner (CCM + CSI + optional autoscaler + upgrades)

This guide covers practical Day-2 components for HKIC-provisioned k3s clusters:

- Hetzner Cloud Controller Manager (CCM)
- Hetzner CSI driver
- Optional: Cluster Autoscaler (Hetzner provider) — see `config/samples/complex/k3s-day2-cluster-autoscaler/README.md`
- Optional: System Upgrade Controller (k3s automated upgrades) — see `config/samples/complex/k3s-day2-system-upgrade/README.md`

These complete a baseline where:

- `Service` type `LoadBalancer` can provision external Hetzner LBs via CCM
- PVCs can dynamically provision volumes via CSI
- (Optional) workers can scale via upstream autoscaler **or** by adding `HCloudServer` workers in GitOps
- (Optional) k3s patch/minor upgrades can roll via SUC `Plan` objects

## Scope

CCM and CSI are the default **baseline**. Cluster Autoscaler and System Upgrade Controller are **optional** parity paths; read each README for interaction with HKIC (especially autoscaler vs `HCloudServer` ownership).

## Prerequisites

- Workload cluster is up and reachable via kubeconfig (for example from `contrib/k3s-optional/export-k3s-kubeconfig-to-secret.sh`).
- `helm` installed locally.
- Hetzner API token with permissions for LB + volume operations.

## 1) Install CCM

Follow:

- `config/samples/complex/k3s-day2-ccm/README.md`

Smoke test:

```bash
kubectl apply -f config/samples/complex/k3s-day2-ccm/lb-smoke-test.yaml
kubectl get svc ccm-lb-smoke -w
```

Expected: service gets external IP.

## 2) Install CSI

Follow:

- `config/samples/complex/k3s-day2-csi/README.md`

Smoke test:

```bash
kubectl apply -f config/samples/complex/k3s-day2-csi/pvc-smoke-test.yaml
kubectl get pvc csi-smoke-pvc -w
kubectl get pod csi-smoke-pod -w
```

Expected: PVC is `Bound`; pod is `Running`.

## 3) Cleanup smoke resources

```bash
kubectl delete -f config/samples/complex/k3s-day2-ccm/lb-smoke-test.yaml
kubectl delete -f config/samples/complex/k3s-day2-csi/pvc-smoke-test.yaml
```

## 4) Optional: Cluster Autoscaler (Hetzner)

Follow `config/samples/complex/k3s-day2-cluster-autoscaler/README.md`. Prefer scaling **`HCloudServer`** workers via GitOps unless you intentionally run the upstream Hetzner autoscaler path in parallel.

## 5) Optional: System Upgrade Controller (k3s)

Follow `config/samples/complex/k3s-day2-system-upgrade/README.md` to install SUC and apply `Plan` objects for server-then-agent upgrades.

## 6) Optional combined verifier

If your cluster was created from the multi-node join sample, you can run one command from your workstation to validate base health plus Day-2 smoke checks remotely on the server node:

```bash
contrib/k3s-optional/verify-k3s-join-cluster.sh k3s-join-server 3 ~/.ssh/id_ed25519 600 true
```

(Requires SSH + management-cluster `kubectl` access as described in `contrib/k3s-optional/README.md`; set `HKC_KUBECONFIG` if your shell uses workload `KUBECONFIG`.)
