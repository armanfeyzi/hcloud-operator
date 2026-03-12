# k3s Day-2: Hetzner CCM

This sample folder provides a practical pattern to install the Hetzner Cloud Controller Manager (CCM) after your k3s cluster is up.

## Prerequisites

- A working kubeconfig for your workload cluster (not the management cluster).
- `helm` installed locally.
- Hetzner API token with permissions to manage load balancers and node metadata.

## 1) Create token Secret in workload cluster

Use `secret-hcloud-token.yaml` and replace `REPLACE_WITH_HCLOUD_TOKEN` first.

```bash
kubectl apply -f config/samples/complex/k3s-day2-ccm/secret-hcloud-token.yaml
```

## 2) Install CCM chart

```bash
helm repo add hcloud https://charts.hetzner.cloud
helm repo update
helm upgrade --install hcloud-ccm hcloud/hcloud-cloud-controller-manager \
  -n kube-system \
  --set env.HCLOUD_TOKEN.valueFrom.secretKeyRef.name=hcloud \
  --set env.HCLOUD_TOKEN.valueFrom.secretKeyRef.key=token
```

If chart values change over time, inspect defaults:

```bash
helm show values hcloud/hcloud-cloud-controller-manager
```

## 3) Verify CCM

```bash
kubectl -n kube-system get pods -l app.kubernetes.io/name=hcloud-cloud-controller-manager
kubectl get nodes -o wide
```

## 4) Optional LoadBalancer smoke test

```bash
kubectl apply -f config/samples/complex/k3s-day2-ccm/lb-smoke-test.yaml
kubectl get svc ccm-lb-smoke -w
```

When CCM is healthy, the Service should eventually get an external IP.
