# hcloud-operator Helm chart

Installs the HKIC operator (Deployment, RBAC, namespace) and all `infra.hkc.io` CRDs from the `crds/` directory.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- Hetzner Cloud API token

## Install

Create the token Secret **before** install (recommended for production):

```bash
kubectl create namespace hcloud-operator-system --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic hcloud-operator-secret \
  -n hcloud-operator-system \
  --from-literal=token="$HCLOUD_TOKEN"
```

From a repository checkout:

```bash
helm install hcloud-operator ./charts/hcloud-operator \
  --namespace hcloud-operator-system \
  --create-namespace
```

Use a released image:

```bash
helm install hcloud-operator ./charts/hcloud-operator \
  --namespace hcloud-operator-system \
  --create-namespace \
  --set image.repository=ghcr.io/armanfeyzi/hcloud-operator \
  --set image.tag=v0.6.2
```

### Dev quickstart (chart creates Secret — do not use in production)

```bash
helm install hcloud-operator ./charts/hcloud-operator \
  --namespace hcloud-operator-system \
  --create-namespace \
  --set secret.create=true \
  --set secret.token="$HCLOUD_TOKEN"
```

## Upgrade

```bash
helm upgrade hcloud-operator ./charts/hcloud-operator -n hcloud-operator-system
```

Helm does not upgrade CRDs on `helm upgrade` by default. After pulling a new chart version, re-apply CRDs if the release notes say so:

```bash
kubectl apply -f charts/hcloud-operator/crds/
```

Or use `helm upgrade --install` with a chart that documents CRD changes.

## Uninstall

```bash
helm uninstall hcloud-operator -n hcloud-operator-system
```

CRDs installed from `crds/` are **not** removed automatically (Helm behaviour). Delete HKIC custom resources first, then optionally:

```bash
kubectl delete -f charts/hcloud-operator/crds/
```

## Values

| Key | Default | Description |
|---|---|---|
| `namespace.name` | `hcloud-operator-system` | Operator namespace |
| `image.repository` | `ghcr.io/armanfeyzi/hcloud-operator` | Container image |
| `image.tag` | `v0.6.2` | Image tag (match GitHub Release) |
| `secret.create` | `false` | Create Secret from `secret.token` |
| `secret.name` | `hcloud-operator-secret` | Secret referenced by Deployment |
| `secret.key` | `token` | Secret data key → `HCLOUD_TOKEN` |
| `leaderElect` | `false` | Controller leader election |

## Maintainer: sync CRDs

When `config/crd/bases/` changes:

```bash
make helm-sync-crds
```
