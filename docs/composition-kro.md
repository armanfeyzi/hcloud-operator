# Composition with kro — HetznerWebStack

HKIC's product is **ACK-style primitives**: one CRD per Hetzner resource. The optional **composition layer** adds a single high-level API, **HetznerWebStack**, implemented as a [kro](https://github.com/kubernetes-sigs/kro) `ResourceGraphDefinition` (RGD).

## What HetznerWebStack expands to

```
HetznerWebStack (instance)
  ├── HCloudNetwork
  ├── HCloudServer      (networkRef → network)
  ├── HCloudVolume      (optional, serverRef → server)
  ├── HCloudLoadBalancer (optional, serverSelector → server labels)
  └── HCloudFirewall    (optional, serverRefs → server)
```

kro orders creation via CEL references between graph nodes. Status fields (server IPv4, LB IPv4, server ID) surface on the stack instance.

## When to use what

| Approach | Best for |
|----------|----------|
| **Raw HKIC CRDs** | Full control, GitOps per resource, arbitrary topologies, production teams that want ACK-style granularity |
| **HetznerWebStack (kro)** | Quick web-tier demos, onboarding, “one YAML” stacks that match `config/samples/complex/hcloud-stack` |
| **Helm (operator chart)** | Installing the **operator** and platform wiring — not a substitute for composing Hetzner resources (use CRDs or kro for that) |

Primitives remain directly usable; kro is additive. Nothing in the operator depends on kro.

## Install

See [composition/kro/README.md](../composition/kro/README.md):

1. `make kro-install` (pinned `KRO_VERSION` in Makefile)
2. Apply HKIC CRDs
3. Apply `composition/kro/hetznerwebstack/rgd.yaml`
4. Apply an instance (`instance-example.yaml`)

## CI validation

CI runs a **kind render test** (no real Hetzner): install kro + HKIC CRDs only, apply RGD + instance, assert child CRs exist with expected spec. See `.github/workflows/composition.yaml`.

## Future follow-ups (same pattern)

- `HetznerK3sCluster` — network + firewall + control plane + agents + LB
- `HetznerDatabaseNode` — server + volume
- Multi-server tiers via kro collections

Design spec: `docs/superpowers/specs/2026-06-07-composition-kro-design.md`.
