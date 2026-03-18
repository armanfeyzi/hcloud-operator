# k3s Day-2 on Hetzner (CCM + CSI)

This guide covers first practical Day-2 components for HKIC-provisioned k3s clusters:

- Hetzner Cloud Controller Manager (CCM)
- Hetzner CSI driver

These complete a baseline where:

- `Service` type `LoadBalancer` can provision external Hetzner LBs via CCM
- PVCs can dynamically provision volumes via CSI

## Scope

This is intentionally a focused baseline. Cluster Autoscaler and System Upgrade Controller are tracked separately in `docs/roadmap.md`.

## Prerequisites

- Workload cluster is up and reachable via kubeconfig (for example from `hack/export-k3s-kubeconfig-to-secret.sh`).
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

## 4) Optional combined verifier

If your cluster was created from the multi-node join sample, you can run one command from your workstation to validate base health plus Day-2 smoke checks remotely on the server node:

```bash
hack/verify-k3s-join-cluster.sh k3s-join-server 3 ~/.ssh/id_ed25519 600 true
```
