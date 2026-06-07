# Design: B — Composition layer (kro `HetznerWebStack`)

Status: Approved (design)
Date: 2026-06-07
Scope: One high-level, optional abstraction (`HetznerWebStack`) implemented as a kro `ResourceGraphDefinition` that renders the existing HKIC CRDs from a single compact spec. Follows the Production Hardening spec (A). No new Go controllers and no operator changes.

## Background and goals

HKIC's vision keeps primitives (one CRD per Hetzner resource) as the product, with composition as an optional layer. Today, composed stacks are expressed as raw multi-CRD sample YAML (`config/samples/complex/*`) plus optional bash helpers. The existing cluster-shape recipe doc explicitly names its "future evolution" as "a higher-level abstraction (Helm or kro) that renders the same CRDs from a single values object."

B delivers that abstraction for the web-stack shape using kro, improving developer experience for complex stacks while keeping the primitives directly usable.

Goal: a single `HetznerWebStack` kind (via a kro RGD) that expands into network + server + volume + load balancer + firewall, with dependency ordering and status surfaced back to the stack instance. Validated in CI without touching real Hetzner.

## Decisions (from brainstorming)

- Mechanism: kro `ResourceGraphDefinition` only (roadmap #16). Not Crossplane, not Helm, for v1.
- Abstractions: only `HetznerWebStack` for v1. `HetznerK3sCluster`, `HetznerDatabaseNode`, `HetznerNetworkBaseline` are explicit future follow-ups using the same pattern.
- k3s bootstrapping: unchanged. B does not modify the operator or the k3s recipes/scripts.
- Depth/done: recipes + docs + CI validation (lint + kind-based render test). No real-Hetzner E2E for B.

## Non-goals

- No new Go controllers, no changes to `internal/controller` or `api/v1alpha1`.
- No Crossplane Compositions.
- No multi-server scaling via kro collections in v1 (single web server).
- No real-Hetzner provisioning in CI for B.

## Architecture

kro is a controller the user installs in their cluster. HKIC ships an RGD (a kro custom resource) that defines a synthetic `HetznerWebStack` CRD. Each `HetznerWebStack` instance is expanded by kro into the existing HKIC CRs:

- `HCloudNetwork` (created first)
- `HCloudServer` (references the network via `networkRef`)
- `HCloudVolume` (references the server via `serverRef`) — optional
- `HCloudLoadBalancer` (selects the server via `serverSelector` labels) — optional
- `HCloudFirewall` (attached by label selector) — optional

kro orders creation using CEL references between graph resources (e.g. the server's `networkRef.name` references the network resource's name). Selected status outputs (load balancer public IPv4, server public IPv4, server ID) are mapped up into the `HetznerWebStack` instance status via the RGD status fields.

The primitives remain fully usable on their own; kro/`HetznerWebStack` is additive and optional.

## `HetznerWebStack` spec (v1)

Compact knobs that fan out into the underlying CRs:

```yaml
apiVersion: kro.run/v1alpha1   # actual group/version pinned in B0
kind: HetznerWebStack
metadata:
  name: demo
spec:
  location: fsn1
  labels: { env: demo }
  network:
    ipRange: 10.90.0.0/16
    zones: [eu-central]
  server:
    type: cx23
    image: ubuntu-22.04
    sshKeys: []
    userData: ""
  volume:            # optional block
    size: 20
    format: ext4
  loadBalancer:      # optional block
    type: lb11
    algorithm: round_robin
    services: []     # listen/target ports + health checks (HCloudLoadBalancer.spec.services shape)
  firewall:          # optional block
    rules: []        # HCloudFirewall.spec.rules shape
```

Mapping to HKIC CRs is 1:1 on field shapes where possible so users can graduate from the abstraction to raw CRDs without relearning field names. Optional blocks, when omitted, simply do not create the corresponding CR.

## Deliverables / files

- Create: `composition/kro/hetznerwebstack/rgd.yaml` — the `ResourceGraphDefinition` (schema + resources + CEL refs + status mapping).
- Create: `composition/kro/hetznerwebstack/instance-example.yaml` — a sample `HetznerWebStack`.
- Create: `composition/kro/README.md` — install kro (pinned), apply RGD, apply instance, observe, cleanup.
- Create: `docs/composition-kro.md` — concept doc + "kro vs raw CRDs vs Helm" guidance.
- Modify: `README.md` (link the composition doc), `docs/roadmap.md` (mark #16 progress).
- CI: lint + kind-based render test (see below).

## CI validation

- Lint/schema: validate `rgd.yaml` and `instance-example.yaml` (e.g. `kubeconform` and/or `kubectl apply --dry-run` against installed CRDs).
- Render test (kind, no real Hetzner):
  1. Spin up `kind`.
  2. Install a pinned kro version.
  3. Install HKIC CRDs only (`config/crd/bases/`) — not the operator, so nothing reconciles against Hetzner.
  4. Apply the RGD, wait for the `HetznerWebStack` CRD to be established.
  5. Apply `instance-example.yaml`.
  6. Assert kro created the expected child CRs (`HCloudNetwork`, `HCloudServer`, plus optional volume/LB/firewall) with the expected spec values and owner references.
  7. Teardown.
- This proves the composition expands correctly without provisioning real infrastructure.

## Testing strategy

- The render test above is the primary gate.
- A small fixture/assertion script (e.g. `kubectl get hcs,hcn,hcv,hclb,hcfw -o jsonpath`) verifies field propagation (location, server type, network ipRange, LB selector matches server labels).

## Risks

- kro is young; RGD API may shift. Mitigation: pin a specific kro version in B0 and document it; design v1 to known-supported features (single instances, simple CEL refs, basic status mapping).
- kro status mapping limits: if surfacing child status proves limited, ship v1 with minimal status (e.g. a readiness summary + LB IP) and note enhancements as follow-up.
- Keep the "primitives usable directly" guarantee — kro stays optional; nothing in the operator depends on it.

## Out of scope / future (same pattern)

- `HetznerK3sCluster` RGD (network + firewall + control-plane + agents + LB).
- `HetznerDatabaseNode` (server + volume), `HetznerNetworkBaseline` (network + firewall).
- Crossplane Compositions of the same shapes.
- Multi-server web tier via kro collections.
