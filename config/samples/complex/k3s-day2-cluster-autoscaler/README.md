# k3s Day-2: Cluster Autoscaler (Hetzner cloud provider)

This folder documents how to run the upstream **Kubernetes Cluster Autoscaler** with **`--cloud-provider=hetzner`**, the same class of setup many **hetzner-k3s** stacks use for worker scale-out.

## HKIC alignment (read this first)

- **HKIC** reconciles **`HCloudServer`** objects in your **management** cluster. The Hetzner autoscaler provider instead **creates and deletes Hetzner servers directly** via the public API using `HCLOUD_TOKEN` and `HCLOUD_CLUSTER_CONFIG` / legacy `HCLOUD_CLOUD_INIT`.
- Those two models can **fight** if both try to own the same servers or labels. Treat autoscaled pools as **explicitly separate** from GitOps-managed nodes (distinct labels / server names / Hetzner project), or prefer **horizontal scale via more `HCloudServer` manifests** until a tighter integration exists.
- This sample is **parity / optional Day-2** documentation, not a requirement to use HKIC.

Upstream reference: [cluster-autoscaler/cloudprovider/hetzner/README.md](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/hetzner/README.md).

## Prerequisites

- Workload kubeconfig (`kubectl` / `KUBECONFIG` pointed at the k3s cluster).
- **CCM** installed so nodes register with provider ID metadata the autoscaler expects (see `../k3s-day2-ccm/README.md`).
- Hetzner API token with permission to create/delete servers (often **read & write** in the same project as the cluster).
- A **join cloud-init** (or `HCLOUD_CLUSTER_CONFIG` JSON) that matches how your nodes bootstrap k3s—copy from a working worker `userData` and adapt pool labels.

## 1) API token Secret

```bash
kubectl apply -f config/samples/complex/k3s-day2-cluster-autoscaler/secret-hcloud-token.yaml
```

Edit `REPLACE_WITH_HCLOUD_TOKEN` before apply (same pattern as CCM/CSI samples).

## 2) Build `HCLOUD_CLUSTER_CONFIG` (recommended)

Prefer the JSON format described in the upstream README (`nodeConfigs`, `imagesForArch`, optional `defaultSubnetIPRange`, …). **Base64-encode** that JSON once (not double-encoded) and expose it to the autoscaler as env **`HCLOUD_CLUSTER_CONFIG`**, or mount **`HCLOUD_CLUSTER_CONFIG_FILE`** from a Secret volume if the JSON is large. See the upstream README for the exact schema and examples.

## 3) Install with Helm

Add the official chart repo and inspect current values (flags change between chart releases):

```bash
helm repo add autoscaler https://kubernetes.github.io/autoscaler
helm repo update
helm show values autoscaler/cluster-autoscaler | less
```

Example install shape (adjust `image.tag` to your Kubernetes **minor** version and set autoscaling group / `--nodes` per upstream Hetzner docs):

```bash
helm upgrade --install cluster-autoscaler autoscaler/cluster-autoscaler \
  -n kube-system \
  --set cloudProvider=hetzner \
  --set extraArgs.stderrthreshold=info \
  --set 'extraArgs.nodes=1:10:cx23:fsn1:pool1'
```

You must also pass **`HCLOUD_TOKEN`** and **`HCLOUD_CLUSTER_CONFIG`** (or legacy `HCLOUD_CLOUD_INIT` / `HCLOUD_IMAGE`) via chart `extraEnv`, `envFromSecret`, or a small wrapper values file—see `values-hetzner.example.yaml` in this directory for a starting point.

## 4) Or install from upstream manifests

Hetzner publishes a runnable Deployment skeleton:

- [cluster-autoscaler-run-on-master.yaml](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/hetzner/examples/cluster-autoscaler-run-on-master.yaml)

Copy it locally, set image tag, `--nodes=…` pools, Secrets, and network/firewall/SSH env vars to match your environment.

## 5) Verify

```bash
kubectl -n kube-system get deploy,po -l app.kubernetes.io/name=cluster-autoscaler
kubectl -n kube-system logs deploy/cluster-autoscaler --tail=50
```

Scale a Deployment that cannot schedule; confirm new nodes appear in **Hetzner Console** and `kubectl get nodes` with your pool labels.
