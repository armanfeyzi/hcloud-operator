# Hetzner Kubernetes Infrastructure Controller (HKIC)

HKIC is a Kubernetes-native infrastructure controller that allows you to manage Hetzner Cloud resources directly from your Kubernetes cluster using Custom Resource Definitions (CRDs).

**Scope:** grow a **wide** set of reconcilers so you can create and configure **many** Hetzner Cloud resource kinds—not only one workflow. **Included in that vision:** standing up a **Kubernetes cluster on Hetzner** (k3s-style, similar to [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s)) by **composing** those controllers, when that is what you need.

Instead of relying on external tools like Terraform or the Hetzner CLI, you can define your infrastructure declaratively as Kubernetes YAML objects. The HKIC operator runs inside your cluster, constantly watching these objects, and automatically reconciles your desired state with the actual infrastructure running in Hetzner Cloud.

## Inspiration and Vision

The long-term vision of this project is to build a lightweight, Kubernetes-native infrastructure layer specifically tailored for Hetzner Cloud. 

This project draws heavy inspiration from:
- **AWS Controllers for Kubernetes (ACK):** The concept of managing cloud-native infrastructure directly through the Kubernetes API.
- **Crossplane:** The abstraction of infrastructure management within Kubernetes, but without the multi-cloud complexity.
- **The Kubernetes Operator Pattern:** Leveraging reconciliation loops to maintain desired state.
- **[hetzner-k3s](https://github.com/vitobotta/hetzner-k3s):** Reference for what a solid **k3s-on-Hetzner** cluster looks like (networking, pools, security, bootstrap, Day-2 add-ons). HKIC aims to cover the **same major Hetzner Cloud components and configs** so you can **deploy, reconfigure, and delete** that class of cluster via CRDs and GitOps—not only ad-hoc VMs.

**How it fits together:** controllers stay **modular** (servers, networks, volumes, load balancers, firewalls, …). You can use one resource type alone or compose them for a full cluster; optional recipes (Helm, kro, samples) stitch the same primitives into an experience similar to `hetzner-k3s create`. See `docs/roadmap.md`.

The goal is not to replace massive multi-cloud tools like Terraform, but rather to provide a focused, lightweight, and tightly integrated experience for users who are already heavily invested in the Kubernetes ecosystem and want to provision Hetzner resources (like Servers and Volumes) without leaving `kubectl`.

### Platform Engineering & KRO Compatibility

Because HKIC uses standard Kubernetes Custom Resources (CRDs), strict validation, and idiomatic `status` fields, it is designed from the ground up to act as a **foundational building block for Internal Developer Platforms (IDPs)**.

You can combine HKIC with orchestrators like [Kube Resource Orchestrator (kro)](https://github.com/kubernetes-sigs/kro) or Crossplane Compositions. This allows Platform Engineers to define higher-level abstractions (e.g., `HetznerDatabaseNode`) that automatically stitch together an `HCloudServer` and an `HCloudVolume` under the hood, hiding infrastructure complexity from developers.

## How It Works

1. **Declarative State:** You apply a custom resource (e.g., `HCloudServer` or `HCloudVolume`) to your cluster.
2. **Reconciliation Loop:** The controller detects the new resource and compares its state against the Hetzner Cloud API.
3. **Provisioning:** If the resource doesn't exist in Hetzner, the controller calls the Hetzner API to create it.
4. **Status Syncing:** The controller syncs important infrastructure details (like IP addresses, volume paths, and Hetzner IDs) back to the `status` field of your Kubernetes resource.
5. **Safe Cleanup:** Resources are protected by Kubernetes Finalizers. When you `kubectl delete` a resource, the controller ensures the physical infrastructure in Hetzner is destroyed before allowing the Kubernetes object to be removed.

## Features

- **Servers (`HCloudServer`):** Provision and manage Hetzner Virtual Machines, including **vertical scaling** (`serverType` changes) via power off → change type → power on, and private network lifecycle via `spec.networkRef` (attach, migrate, detach).
- **Networks (`HCloudNetwork`):** Provision private networks and optional per-zone Cloud subnets (`networkZones`).
- **Volumes (`HCloudVolume`):** Provision block storage and automatically attach it to your servers using Kubernetes native references (`serverRef`).
- **Load Balancers (`HCloudLoadBalancer`):** Expose selected servers through a public Hetzner Load Balancer using `serverSelector` label matching.
- **Firewalls (`HCloudFirewall`):** Define Hetzner Cloud firewall rules and attach to servers by `HCloudServer` reference and/or a Hetzner label selector.
- **Idempotent Operations:** The controller is designed to handle API interruptions safely without creating duplicate infrastructure.

## Quick Start

### 1. Prerequisites
- A running Kubernetes cluster (or `kind`/`minikube` for local testing)
- A Hetzner Cloud API Token

### 2. Install CRDs (required for every cluster)

Apply **all** CRD manifests whenever you upgrade the operator or use resources such as volumes and load balancers—not only `HCloudServer`:

```bash
# From a checkout of this repository (recommended)
make install
# Equivalent:
kubectl apply -f config/crd/bases/
```

If you skip this step, `kubectl apply` may create `HCloudServer` objects but fail on `HCloudVolume` / `HCloudLoadBalancer` / `HCloudNetwork` / `HCloudFirewall` with “resource mapping not found”.

### 3. Production Installation (operator in-cluster)

Create the operator namespace and store your Hetzner token:

```bash
kubectl create namespace hcloud-operator-system --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic hcloud-operator-secret \
  -n hcloud-operator-system \
  --from-literal=token=your-api-token
```

The Deployment expects the key **`token`** (it maps to the `HCLOUD_TOKEN` environment variable inside the container).

**Option A — released install (recommended, matches CI):** every Git tag build publishes `install.yaml` with the image set to `ghcr.io/<repo>:<tag>`.

```bash
export VERSION=v0.6.0   # or use latest: see GitHub Releases
kubectl apply -f "https://github.com/armanfeyzi/hcloud-operator/releases/download/${VERSION}/install.yaml"
# or: make deploy-release VERSION=v0.6.0
```

**Option B — from a git clone** (local dev / custom `IMG`):

```bash
# Same secret as above, then either local kustomize (localhost image) or a published image:
make deploy-img IMG=ghcr.io/armanfeyzi/hcloud-operator:v0.6.0
# or: kubectl apply -k config/default/   # after docker build + kind load, or with IMG substitution
```

**Option C — latest release URL**

```bash
kubectl apply -f https://github.com/armanfeyzi/hcloud-operator/releases/latest/download/install.yaml
```

If that URL returns 404, no release exists yet—build from a clone or run `make run` locally instead.

**Troubleshooting — pod `CreateContainerConfigError` / `couldn't find key token`:** the Secret must use **`token`**, not `HCLOUD_TOKEN`. Fix with:
`kubectl create secret generic hcloud-operator-secret -n hcloud-operator-system --from-literal=token="$HCLOUD_TOKEN" --dry-run=client -o yaml | kubectl apply -f -`

Releases **before this repo fix** shipped `install.yaml` built from a CRD kustomization that did not yet list `HCloudFirewall`; if `kubectl api-resources | grep hcloudfirewalls` is empty after applying a release, apply the firewall CRD from a checkout (`kubectl apply -f config/crd/bases/infra.hkc.io_hcloudfirewalls.yaml`) or upgrade to a newer tag once published.

### 4. Local Development Install
If you want to run the operator locally on your machine against your cluster:

```bash
make install
export HCLOUD_TOKEN="your-api-token"
make run
```

### 5. Create Infrastructure

Define a private network, a server attached to it, and a volume attached to the server:

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudNetwork
metadata:
  name: app-private-net
spec:
  ipRange: 10.80.0.0/16
  networkZones:
    - eu-central
---
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudServer
metadata:
  name: database-node
spec:
  serverType: cx21
  image: ubuntu-22.04
  location: fsn1
  networkRef:
    name: app-private-net
---
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudVolume
metadata:
  name: database-storage
spec:
  size: 50
  format: ext4
  serverRef:
    name: database-node # Automatically attaches to the server above!
```

Expose selected servers via a load balancer:

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudLoadBalancer
metadata:
  name: public-web
spec:
  loadBalancerType: lb11
  location: fsn1
  algorithm: round_robin
  serverSelector:
    matchLabels:
      app: web
```

Apply it to your cluster:
```bash
kubectl apply -f infra.yaml
```

A ready-to-apply version of this pattern (server + volume + load balancer) lives at `config/samples/complex/hcloud-stack/hcloud_stack_v1alpha1.yaml`.

Check the status to see the assigned IP addresses, load balancer front-end, and mount paths:
```bash
kubectl get hcs,hcv,hclb
```

## Documentation

- [Architecture](docs/architecture.md)
- [CRD Reference](docs/crds.md)
- [Development Guide](docs/development.md)
- [k3s on Hetzner with HKIC](docs/k3s-on-hcloud.md) — cloud-init sample cluster
- [k3s Day-2 (CCM + CSI)](docs/k3s-day2.md)
- [hetzner-k3s `cluster.yaml` -> HKIC mapping](docs/hetzner-k3s-cluster-yaml-mapping.md)
- [k3s kubeconfig access pattern](docs/k3s-on-hcloud.md#secure-kubeconfig-handling-recommended)
- [Roadmap](docs/roadmap.md)
- [Contributing](CONTRIBUTING.md)

## License
MIT License