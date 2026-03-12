# k3s Day-2: Hetzner CSI

This sample folder provides a practical pattern to install the Hetzner CSI driver and validate dynamic volume provisioning.

## Prerequisites

- A working kubeconfig for your workload cluster (not the management cluster).
- `helm` installed locally.
- Hetzner API token with storage permissions.

## 1) Create token Secret in workload cluster

Use `secret-hcloud-token.yaml` and replace `REPLACE_WITH_HCLOUD_TOKEN` first.

```bash
kubectl apply -f config/samples/complex/k3s-day2-csi/secret-hcloud-token.yaml
```

## 2) Install CSI chart

```bash
helm repo add hcloud https://charts.hetzner.cloud
helm repo update
helm upgrade --install hcloud-csi hcloud/hcloud-csi \
  -n kube-system \
  --set controller.hcloudToken.existingSecret.name=hcloud \
  --set controller.hcloudToken.existingSecret.key=token \
  --set node.hcloudToken.existingSecret.name=hcloud \
  --set node.hcloudToken.existingSecret.key=token
```

If chart values change over time, inspect defaults:

```bash
helm show values hcloud/hcloud-csi
```

## 3) Verify CSI

```bash
kubectl -n kube-system get pods -l app.kubernetes.io/name=hcloud-csi
kubectl get csinodes
kubectl get storageclass
```

## 4) PVC smoke test

```bash
kubectl apply -f config/samples/complex/k3s-day2-csi/pvc-smoke-test.yaml
kubectl get pvc csi-smoke-pvc -w
kubectl get pod csi-smoke-pod -w
```

Expected:

- PVC becomes `Bound`
- Pod becomes `Running`
- A dynamic Hetzner volume is provisioned and attached through CSI
