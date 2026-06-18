# Project Roadmap

The **primary** goal of **Hetzner Kubernetes Infrastructure Controller (HKIC)** is an **[ACK](https://aws.amazon.com/blogs/containers/aws-controllers-for-kubernetes-ack/)-style** experience for **Hetzner Cloud**: broad **CRD + reconciler** coverage so teams can declare cloud resources in Kubernetes and reconcile them with GitOps (Argo CD, Flux, and so on) like any other workload—**simple single resources and composed stacks** without a separate Hetzner-specific workflow for day-to-day changes.

**Optional (not the product goal):** samples and docs showing how those **same CRDs** can be composed into higher-level outcomes—Kubernetes-on-Hetzner (k3s), multi-tier apps, platform stacks—via `userData`, Day-2 manifests, optional scripts in `contrib/k3s-optional/`, Helm, or [kro](https://github.com/kubernetes-sigs/kro). Users build whatever they need from primitives; k3s is **one recipe among many**, not a north star.

**Two layers:**

1. **Hetzner Cloud API coverage (main track):** add and harden CRDs/reconcilers for the resource types teams use (servers, networks, volumes, load balancers, firewalls, placement groups, …). Users adopt **any subset**. Progress: [docs/hcloud-api-coverage.md](hcloud-api-coverage.md).
2. **Composition recipes (optional track):** examples that wire primitives together. Includes k3s/hetzner-k3s-oriented docs under `docs/k3s-*` and `config/samples/complex/k3s-*`.

## Current focus — Milestone 2: Operator & GitOps platform

**Milestone 1 (Hetzner Cloud CRD coverage) is complete** except optional deferred work ([#8](https://github.com/armanfeyzi/hcloud-operator/issues/8) SSH keys). Production GitOps adoption now depends on **install paths and examples**, not new resource CRDs.

Tracked on GitHub milestone **[HKIC: Operator & GitOps platform](https://github.com/armanfeyzi/hcloud-operator/milestone/2)** (issues #11–#16). **Next:** [#15 multi-cluster docs](https://github.com/armanfeyzi/hcloud-operator/issues/15) and optional manual GitOps smoke tests — see [docs/sprint-platform-gitops.md](sprint-platform-gitops.md).

| Priority | Work | Issue |
|---|---|---|
| **High (now)** | Multi-cluster / multi-token patterns (documentation) | #15 |
| Out of scope (repo) | kro / Crossplane abstractions — users compose HKIC CRDs in their own tooling ([#16](https://github.com/armanfeyzi/hcloud-operator/issues/16) discussion only) |
| Deferred (M1) | `HCloudSSHKey` CRD — reference-by-name on Server is enough for most teams | #8 |
| **Complete** | Helm chart — operator image, token secret, values for install/upgrade | #12 |
| **Complete** | Argo CD Application(s) — sync waves for a full HKIC stack | #13 |
| **Complete** | Observability — Events, Prometheus metrics, richer conditions; shared base reconciler (**all nine controllers migrated**) | #11 |
| **Complete** | Opt-in CI E2E against real Hetzner (`e2e_real` build tag, `make test-e2e-real`) | #14 |

## Milestone 1 — Hetzner Cloud API coverage — **Complete**

Tracked on GitHub milestone **[HKIC: Hetzner Cloud CRD coverage](https://github.com/armanfeyzi/hcloud-operator/milestone/1)** (issues #1–#9). All planned CRDs and hardening for this milestone are **shipped**; only optional SSH key lifecycle remains open.

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

- [ ] **`HCloudSSHKey`** — Hetzner API object exists, but ACK-style stacks typically reference existing key **names** on `HCloudServer.spec.sshKeys`; dedicated CRD only if fully declarative key lifecycle is required (#8)

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

## Phase 4: GitOps & platform engineering — **Current focus**
- [x] **Helm chart** — [charts/hcloud-operator/](../charts/hcloud-operator/README.md), `make helm-lint` (#12)
- [x] **Argo CD examples** — [examples/argo/](../examples/argo/README.md) (#13)
- [x] **CI E2E** against real Hetzner when `HCLOUD_TOKEN` is present in repo secrets (#14) — `.github/workflows/e2e-real.yaml`, `make test-e2e-real`
- [ ] Multi-cluster / multi-token patterns (documentation first) (#15)
- [ ] Higher-level abstractions ([kro](https://github.com/kubernetes-sigs/kro), Crossplane, …) — **not shipped in this repo**; use `config/samples/complex/*` as raw-CRD composition baselines (#16)

## Optional: k3s-on-Hetzner composition recipes

**Not the operator goal** — optional documentation and samples for teams that want to build k3s (or similar) from HKIC primitives. Tracked loosely under **[HKIC: Parity & verification](https://github.com/armanfeyzi/hcloud-operator/milestone/3)** (#10).

- [x] k3s sample + doc — [docs/k3s-on-hcloud.md](k3s-on-hcloud.md), `config/samples/complex/k3s-*`
- [x] Node bootstrap helpers — `contrib/k3s-optional/`
- [x] Cluster-shape recipe — [docs/k3s-cluster-shape-recipe.md](k3s-cluster-shape-recipe.md)
- [x] Day-2 manifests — [docs/k3s-day2.md](k3s-day2.md), `config/samples/complex/k3s-day2-*`
- [x] hetzner-k3s mapping — [docs/hetzner-k3s-cluster-yaml-mapping.md](hetzner-k3s-cluster-yaml-mapping.md)
- [ ] **Optional parity checklist / E2E runbook** (#10) — only if we want automated verification of the k3s recipe

## Hygiene & contributor experience
- [x] Envtest isolation: drain cluster CRs + reset shared `FakeClient` between Ginkgo specs (random order safe)
- [x] Keep **`.idea/project-idea.md`** and **`.idea/AGENT_*.md`** aligned with README / roadmap when major features land (north star: **ACK-style Hetzner API coverage**)

## GitHub tracking

Work is tracked on GitHub under **[Milestones](https://github.com/armanfeyzi/hcloud-operator/milestones)**:

| Milestone | Focus | Status |
|---|---|---|
| **HKIC: Hetzner Cloud CRD coverage** | Main ACK track — CRDs + hardening (#1–#9) | **Complete** (optional #8 deferred) |
| **HKIC: Operator & GitOps platform** | Helm, Argo, observability, CI (#11–#16) | **Active — current focus** |
| **HKIC: Parity & verification** | Optional k3s recipe verification (#10) | Optional |
