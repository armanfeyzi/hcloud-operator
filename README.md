# Hetzner Kubernetes Infrastructure Controller (HKIC)

HKIC is a **Kubernetes-native controller for Hetzner Cloud**, in the same spirit as **[AWS Controllers for Kubernetes (ACK)](https://aws.amazon.com/blogs/containers/aws-controllers-for-kubernetes-ack/)**: you declare Hetzner resources as **CRDs**, the operator reconciles them against the **Hetzner API**, and you read back **observed state in `status`**—no Hetzner-specific CLI required for day-to-day use.

### What you can do (simple → advanced)

1. **Single resources** — e.g. one `HCloudServer`, one `HCloudVolume`, one `HCloudNetwork` (same GitOps patterns as any other Kubernetes object).
2. **Composed stacks** — `config/samples/complex/*` shows how to wire primitives together (server + network + volume + load balancer + firewall).
3. **Optional: Kubernetes on Hetzner** — still **using the same CRDs** (`HCloudServer` + `userData`, networks, firewalls, …), with docs/samples that approximate what [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) gives you. That path is **composition and documentation**, not a second product hiding the CRDs.

### GitOps and Argo CD (why this is not “just hetzner-k3s”)

Because everything is normal Kubernetes API objects, you can put HKIC resources in the **same GitOps repo and Argo CD Applications** as your applications: diff, sync, rollback, and observe health in the **same dashboards and workflows** you already use for apps. Infra is not a one-off CLI run—it is versioned, reviewable, and continuous, whether you deploy one server or a full cluster-shaped stack.

The operator runs in your management cluster, watches those CRDs, and converges Hetzner Cloud to match.

## Inspiration and Vision

Primary inspiration:

- **[AWS Controllers for Kubernetes (ACK)](https://aws.amazon.com/blogs/containers/aws-controllers-for-kubernetes-ack/):** one cloud, many resource kinds, each reconciled through the Kubernetes API.
- **The Kubernetes operator pattern:** idempotent reconcile loops, finalizers, and status.

Related ideas (not the core product definition):

- **Crossplane-style composition** — optional; HKIC stays useful as plain CRDs.
- **[hetzner-k3s](https://github.com/vitobotta/hetzner-k3s):** referenced only in **optional** k3s composition samples—not a product goal or runtime dependency. The main track is **Hetzner Cloud API coverage** ([coverage matrix](docs/hcloud-api-coverage.md)).

**How it fits together:** one reconciler per concern (`HCloudServer`, `HCloudNetwork`, …). You adopt only what you need. See `docs/roadmap.md` for API coverage and optional cluster-shaped work.

The goal is not to replace every multi-cloud tool, but to give teams that already live in **Kubernetes + GitOps** a first-class way to own **Hetzner infrastructure** the same way they own workloads.

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

- **Servers (`HCloudServer`):** Provision and manage Hetzner Virtual Machines, including **vertical scaling** (`serverType` changes) with optional **`upgradeDisk`**, and private network lifecycle via `spec.networkRef` (attach, migrate, detach).
- **Networks (`HCloudNetwork`):** Provision private networks and optional per-zone Cloud subnets (`networkZones`).
- **Volumes (`HCloudVolume`):** Provision block storage, **resize up** via `spec.size`, and attach to servers using `serverRef`.
- **Load Balancers (`HCloudLoadBalancer`):** Expose selected servers through a public Hetzner Load Balancer using `serverSelector`, with **services and health checks** via `spec.services`.
- **Firewalls (`HCloudFirewall`):** Define Hetzner Cloud firewall rules and attach to servers by `HCloudServer` reference and/or a Hetzner label selector.
- **Placement Groups (`HCloudPlacementGroup`):** Spread or cluster placement groups; reference from `HCloudServer.spec.placementGroupRef` at server creation.
- **Primary IPs (`HCloudPrimaryIP`):** Allocate IPv4/IPv6 primary IPs, assign to servers via `serverRef`, set reverse DNS, and configure `autoDelete`.
- **Floating IPs (`HCloudFloatingIP`):** Allocate IPv4/IPv6 floating IPs by location, assign to servers via `serverRef`, set reverse DNS, and sync description/labels.
- **Idempotent Operations:** The controller is designed to handle API interruptions safely without creating duplicate infrastructure.

## Where to start

- **ACK-style use (single resources):** `config/samples/simple/` and [CRD reference](docs/crds.md).
- **Composed Hetzner stacks:** [Sample index](config/samples/README.md) and e.g. `config/samples/complex/hcloud-stack/`.
- **Optional k3s / bootstrap helpers:** [k3s on Hetzner](docs/k3s-on-hcloud.md) and [contrib/k3s-optional/](contrib/k3s-optional/README.md) are **convenience** around `spec.userData` and SSH—they are **not** required to operate the controller or to manage primitives via GitOps.

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
export VERSION=v0.6.2   # or use latest: see GitHub Releases
kubectl apply -f "https://github.com/armanfeyzi/hcloud-operator/releases/download/${VERSION}/install.yaml"
# or: make deploy-release VERSION=v0.6.2
```

**Option B — from a git clone** (local dev / custom `IMG`):

```bash
# Same secret as above, then either local kustomize (localhost image) or a published image:
make deploy-img IMG=ghcr.io/armanfeyzi/hcloud-operator:v0.6.2
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
- [Optional k3s SSH / join scripts](contrib/k3s-optional/README.md) — cookbook only, not part of the operator
- [k3s Day-2 (CCM + CSI)](docs/k3s-day2.md)
- [hetzner-k3s `cluster.yaml` -> HKIC mapping](docs/hetzner-k3s-cluster-yaml-mapping.md)
- [k3s cluster-shape composition recipe](docs/k3s-cluster-shape-recipe.md)
- [k3s kubeconfig access pattern](docs/k3s-on-hcloud.md#secure-kubeconfig-handling-recommended)
- [Hetzner Cloud API coverage matrix](docs/hcloud-api-coverage.md)
- [Roadmap](docs/roadmap.md)
- [Contributing](CONTRIBUTING.md)

## License
MIT License