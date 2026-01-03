# 🚀 Hetzner Kubernetes Infrastructure Controller (HKIC)

A Kubernetes-native infrastructure controller for Hetzner Cloud.

Instead of using Terraform or CLI tools, HKIC lets you manage Hetzner Cloud infrastructure natively using Kubernetes Custom Resources (CRDs). Define your infrastructure declaratively in YAML, and let the operator reconcile it.

## 🎯 Features (MVP)
- Define Hetzner Servers via `HCloudServer` CRDs
- Automatic creation and state synchronization
- Finalizer-based safe deletion

## 📦 Quick Start

### 1. Prerequisites
- A Kubernetes cluster
- Hetzner Cloud API Token (`HCLOUD_TOKEN`)

### 2. Production Installation (GitOps/kubectl)
To install the operator into a real cluster, deploy the release manifest. Ensure you set your Hetzner token in the `hcloud-operator-system` namespace first:

```bash
kubectl create namespace hcloud-operator-system
kubectl create secret generic hcloud-operator-secret \
  -n hcloud-operator-system \
  --from-literal=HCLOUD_TOKEN=your-api-token

kubectl apply -f https://github.com/armanfeyzi/hcloud-operator/releases/download/v0.1.1/install.yaml
```

### 3. Local Development Install
If you're developing the operator locally:
```bash
make install
export HCLOUD_TOKEN="your-api-token"
make run
```

### 4. Create a Server
```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudServer
metadata:
  name: dev-node-1
spec:
  serverType: cx23
  image: ubuntu-22.04
  location: fsn1
```

```bash
kubectl apply -f config/samples/hcloudserver_v1alpha1.yaml
```

## 📖 Documentation
- [Architecture](docs/architecture.md)
- [CRD Reference](docs/crds.md)
- [Development Guide](docs/development.md)
- [Roadmap](docs/roadmap.md)

## ⚖️ License
MIT License