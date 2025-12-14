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

### 2. Install the CRDs
```bash
make install
```

### 3. Run the Operator Locally
```bash
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