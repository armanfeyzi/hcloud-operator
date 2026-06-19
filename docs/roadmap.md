# Project Roadmap

The **primary** goal of **Hetzner Kubernetes Infrastructure Controller (HKIC)** is an **[ACK](https://aws.amazon.com/blogs/containers/aws-controllers-for-kubernetes-ack/)-style** experience for **Hetzner Cloud**: broad **CRD + reconciler** coverage so teams can declare cloud resources in Kubernetes and reconcile them with GitOps (Argo CD, Flux, and so on) like any other workload—**simple single resources and composed stacks** without a separate Hetzner-specific workflow for day-to-day changes.

**Optional (not the product goal):** samples and docs showing how those **same CRDs** can be composed into higher-level outcomes—Kubernetes-on-Hetzner (k3s), multi-tier apps, platform stacks—via `userData`, Day-2 manifests, optional scripts in `contrib/k3s-optional/`, Helm, or [kro](https://github.com/kubernetes-sigs/kro). Users build whatever they need from primitives; k3s is **one recipe among many**, not a north star.

**Two layers:**

1. **Hetzner Cloud API coverage (main track):** add and harden CRDs/reconcilers for the resource types teams use (servers, networks, volumes, load balancers, firewalls, placement groups, …). Users adopt **any subset**. Progress: [docs/hcloud-api-coverage.md](hcloud-api-coverage.md).
2. **Composition recipes (optional track):** examples that wire primitives together. Includes k3s/hetzner-k3s-oriented docs under `docs/k3s-*` and `config/samples/complex/k3s-*`.

## Current focus — Platform docs & optional work

**Milestones 1–2 are complete** (CRD coverage + GitOps platform). Optional deferred work (`HCloudSSHKey`) remains low priority.

**Task tracking:** [Linear — HCloud Operator project](https://linear.app/armanfeyzi/project/hcloud-operator-a88af6f6c1e8). GitHub Issues are closed; use Linear for active roadmap work.

| Priority | Work | Linear |
|---|---|---|
| **High (now)** | Multi-cluster / multi-token patterns (documentation) | [ARM-71](https://linear.app/armanfeyzi/issue/ARM-71) |
| Deferred | `HCloudSSHKey` CRD — reference-by-name on Server is enough for most teams | [ARM-72](https://linear.app/armanfeyzi/issue/ARM-72) |
| Optional | k3s parity checklist / E2E runbook | [ARM-73](https://linear.app/armanfeyzi/issue/ARM-73) |
| Out of scope (repo) | kro / Crossplane abstractions — users compose HKIC CRDs in their own tooling | — |
| **Complete** | Milestone 2 — observability, base reconciler, leader election, real-Hetzner E2E | [ARM-30](https://linear.app/armanfeyzi/issue/ARM-30) |

## Milestone 1 — Hetzner Cloud API coverage — **Complete**

All planned CRDs and hardening for this milestone are **shipped**; optional SSH key lifecycle is deferred in Linear ([ARM-72](https://linear.app/armanfeyzi/issue/ARM-72)).

### Shipped (v1alpha1)

- [x] `HCloudServer` — create / delete / adopt-by-name, type changes, `networkRef`, optional `upgradeDisk`
- [x] `HCloudVolume` — lifecycle, attach via `serverRef`, **resize**
- [x] `HCloudLoadBalancer` — lifecycle, target sync, **services + health checks**, TLS via `certificateRefs`
- [x] `HCloudNetwork` — private networks, Cloud subnets
- [x] `HCloudFirewall` — rules, attach via `serverRefs` / label selector
- [x] `HCloudPlacementGroup` — spread/cluster groups; `HCloudServer.spec.placementGroupRef` (#2, #3)
- [x] `HCloudPrimaryIP` — IPv4/IPv6, datacenter, server assignment, DNS PTR (#4)
- [x] `HCloudFloatingIP` — IPv4/IPv6, location, server assignment, DNS PTR (#5)
- [x] `HCloudCertificate` — uploaded/managed TLS certs; HTTPS listeners via `certificateRefs` (#6)
- [x] **API coverage matrix** — [docs/hcloud-api-coverage.md](hcloud-api-coverage.md) (#1)
- [x] **Volume snapshots** — **N/A**; Hetzner Cloud has no volume snapshot API (#7 closed)
- [x] **Harden existing CRDs** — LB listeners/health, `upgradeDisk` on server, volume resize (#9)

### Deferred (optional, low priority)

- [ ] **`HCloudSSHKey`** — optional; track in Linear [ARM-72](https://linear.app/armanfeyzi/issue/ARM-72) if fully declarative key lifecycle is required

## Phase 1: MVP — **Complete**
- [x] Project scaffolding
- [x] `HCloudServer` CRD and reconciler (create / delete / adopt-by-name)
- [x] Status sync (IPs, state, conditions)
- [x] Finalizer-based cleanup

## Phase 2: Compute & storage — **Complete**
- [x] Server vertical scaling (`spec.serverType` changes via power off → `change_type` → power on; `status.appliedServerType`)
- [x] Optional **`upgradeDisk`** on type change (`HCloudServer.spec.upgradeDisk`)
- [x] `HCloudVolume` CRD and reconciler
- [x] Volume attach / detach via `serverRef` and drift correction
- [x] `HCloudLoadBalancer` CRD with `serverSelector` and target sync
- [x] Load balancer reconciler watches `HCloudServer` so label / status changes re-sync targets without waiting for periodic requeue
- [x] **`Watches` on `HCloudServer` from `HCloudVolume`** — re-attach as soon as `status.serverID` appears
- [x] **Conflict-safe status updates** — retry-on-conflict for controller status writes
- [x] **Observability** — Kubernetes `Events`, Prometheus metrics (`internal/metrics` + `hcloud.Instrument()` API wrapper), richer conditions (#11). Shared generic base reconciler (`internal/reconcile`) owns the loop skeleton, `Synced` condition, Events, and reconcile metrics; **all nine controllers** use the base. See [docs/observability.md](observability.md). Leader election defaults **on**.

## Phase 3: Networking — **Complete**
- [x] `HCloudNetwork` CRD and reconciler
- [x] Attach Cloud Servers to `HCloudNetwork` (`HCloudServer.spec.networkRef`)
- [x] `HCloudFirewall` CRD and reconciler

## Phase 4: GitOps & platform engineering — **Complete** (active docs in Linear)
- [x] **Helm chart** — [charts/hcloud-operator/](../charts/hcloud-operator/README.md), `make helm-lint`
- [x] **Argo CD examples** — [examples/argo/](../examples/argo/README.md)
- [x] **CI E2E** against real Hetzner — `.github/workflows/e2e-real.yaml`, `make test-e2e-real`
- [ ] Multi-cluster / multi-token patterns — [ARM-71](https://linear.app/armanfeyzi/issue/ARM-71)
- [x] Higher-level abstractions (kro, Crossplane) — **not shipped in this repo**; use `config/samples/complex/*` as raw-CRD baselines

## Optional: k3s-on-Hetzner composition recipes

**Not the operator goal** — optional documentation and samples for teams that want to build k3s (or similar) from HKIC primitives. Optional verification: Linear [ARM-73](https://linear.app/armanfeyzi/issue/ARM-73).

- [x] k3s sample + doc — [docs/k3s-on-hcloud.md](k3s-on-hcloud.md), `config/samples/complex/k3s-*`
- [x] Node bootstrap helpers — `contrib/k3s-optional/`
- [x] Cluster-shape recipe — [docs/k3s-cluster-shape-recipe.md](k3s-cluster-shape-recipe.md)
- [x] Day-2 manifests — [docs/k3s-day2.md](k3s-day2.md), `config/samples/complex/k3s-day2-*`
- [x] hetzner-k3s mapping — [docs/hetzner-k3s-cluster-yaml-mapping.md](hetzner-k3s-cluster-yaml-mapping.md)
- [ ] **Optional parity checklist / E2E runbook** — [ARM-73](https://linear.app/armanfeyzi/issue/ARM-73)

## Hygiene & contributor experience
- [x] Envtest isolation: drain cluster CRs + reset shared `FakeClient` between Ginkgo specs (random order safe)
- [x] Keep **`.idea/project-idea.md`** and **`.idea/AGENT_*.md`** aligned with README / roadmap when major features land (north star: **ACK-style Hetzner API coverage**)

## Task tracking (Linear)

Active roadmap and implementation tasks live in the **[HCloud Operator](https://linear.app/armanfeyzi/project/hcloud-operator-a88af6f6c1e8)** Linear project.

GitHub Issues and milestones are **closed** (historical reference only). External contributors may still open GitHub Issues for bugs/PRs; maintainers triage into Linear.
