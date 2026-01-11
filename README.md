# Hetzner Kubernetes Infrastructure Controller (HKIC)

HKIC is a Kubernetes-native infrastructure controller that allows you to manage Hetzner Cloud resources directly from your Kubernetes cluster using Custom Resource Definitions (CRDs).

Instead of relying on external tools like Terraform or the Hetzner CLI, you can define your infrastructure declaratively as Kubernetes YAML objects. The HKIC operator runs inside your cluster, constantly watching these objects, and automatically reconciles your desired state with the actual infrastructure running in Hetzner Cloud.

## Inspiration and Vision

The long-term vision of this project is to build a lightweight, Kubernetes-native infrastructure layer specifically tailored for Hetzner Cloud. 

This project draws heavy inspiration from:
- **AWS Controllers for Kubernetes (ACK):** The concept of managing cloud-native infrastructure directly through the Kubernetes API.
- **Crossplane:** The abstraction of infrastructure management within Kubernetes, but without the multi-cloud complexity.
- **The Kubernetes Operator Pattern:** Leveraging reconciliation loops to maintain desired state.

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

- **Servers (`HCloudServer`):** Provision and manage Hetzner Virtual Machines.
- **Volumes (`HCloudVolume`):** Provision block storage and automatically attach it to your servers using Kubernetes native references (`serverRef`).
- **Load Balancers (`HCloudLoadBalancer`):** Expose selected servers through a public Hetzner Load Balancer using `serverSelector` label matching.
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

If you skip this step, `kubectl apply` may create `HCloudServer` objects but fail on `HCloudVolume` / `HCloudLoadBalancer` with “resource mapping not found”.

### 3. Production Installation (operator in-cluster)

Create the operator namespace and store your Hetzner token:

```bash
kubectl create namespace hcloud-operator-system
kubectl create secret generic hcloud-operator-secret \
  -n hcloud-operator-system \
  --from-literal=HCLOUD_TOKEN=your-api-token
```

Then install the bundled manifests from your checkout (builds the same layout as the release workflow):

```bash
kubectl apply -k config/default/
```

When you publish a GitHub release, the workflow attaches `install.yaml`. You can then use:

```bash
kubectl apply -f https://github.com/armanfeyzi/hcloud-operator/releases/latest/download/install.yaml
```

If that URL returns 404, no release exists yet—use `kubectl apply -k config/default/` from a clone instead.

### 4. Local Development Install
If you want to run the operator locally on your machine against your cluster:

```bash
make install
export HCLOUD_TOKEN="your-api-token"
make run
```

### 5. Create Infrastructure

Define a server and an attached volume:

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudServer
metadata:
  name: database-node
spec:
  serverType: cx21
  image: ubuntu-22.04
  location: fsn1
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

A ready-to-apply version of this pattern (server + volume + load balancer) lives at `config/samples/hcloud_stack_v1alpha1.yaml`.

Check the status to see the assigned IP addresses, load balancer front-end, and mount paths:
```bash
kubectl get hcs,hcv,hclb
```

## Documentation

- [Architecture](docs/architecture.md)
- [CRD Reference](docs/crds.md)
- [Development Guide](docs/development.md)
- [Roadmap](docs/roadmap.md)
- [Contributing](CONTRIBUTING.md)

## License
MIT License