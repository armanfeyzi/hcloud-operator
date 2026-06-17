# kro composition — HetznerWebStack

Optional [kro](https://github.com/kubernetes-sigs/kro) (`ResourceGraphDefinition`) layer that expands a single **HetznerWebStack** instance into the existing HKIC CRDs.

## Prerequisites

- Kubernetes cluster with HKIC CRDs installed (`make install` or Helm `crds/`)
- **kro** controller (pinned in the root `Makefile` as `KRO_VERSION`)
- For real provisioning: HKIC operator running with `HCLOUD_TOKEN`

kro is **not** required for direct HKIC CRD usage — primitives remain fully usable on their own.

## Quick start

```bash
# Pin: KRO_VERSION=v0.9.2 (see Makefile)
make kro-install

kubectl apply -f config/crd/bases/
kubectl apply -f composition/kro/hetznerwebstack/rgd.yaml

# Wait for RGD Active / HetznerWebStack CRD established
kubectl get rgd hetznerwebstack -o wide

kubectl apply -f composition/kro/hetznerwebstack/instance-example.yaml
kubectl get hetznerwebstack demo -o wide
kubectl get hcn,hcs,hcv,hclb -l hetznerwebstack.hkc.io/stack=demo
```

## Layout

| Path | Purpose |
|------|---------|
| `hetznerwebstack/rgd.yaml` | ResourceGraphDefinition (schema + graph) |
| `hetznerwebstack/instance-example.yaml` | Copy-paste demo instance |
| `test/assert.sh` | kind render-test assertions (CI) |

## Cleanup

```bash
kubectl delete -f composition/kro/hetznerwebstack/instance-example.yaml --ignore-not-found
kubectl delete -f composition/kro/hetznerwebstack/rgd.yaml --ignore-not-found
```

## Further reading

- [docs/composition-kro.md](../../docs/composition-kro.md) — when to use kro vs raw CRDs vs Helm
- [config/samples/complex/hcloud-stack/](../../config/samples/complex/hcloud-stack/) — equivalent multi-CRD sample
