# Project Roadmap

The **primary** goal of **Hetzner Kubernetes Infrastructure Controller (HKIC)** is an **[ACK](https://aws.amazon.com/blogs/containers/aws-controllers-for-kubernetes-ack/)-style** experience for **Hetzner Cloud**: broad **CRD + reconciler** coverage so teams can declare cloud resources in Kubernetes and reconcile them with GitOps (Argo CD, Flux, and so on) like any other workload—**simple single resources and composed stacks** without a separate Hetzner-specific workflow for day-to-day changes.

**Optional (not the product goal):** samples and docs showing how those **same CRDs** can be composed into higher-level outcomes—Kubernetes-on-Hetzner (k3s), multi-tier apps, platform stacks—via `userData`, Day-2 manifests, optional scripts in `contrib/k3s-optional/`, Helm, or [kro](https://github.com/kubernetes-sigs/kro). Users build whatever they need from primitives; k3s is **one recipe among many**, not a north star.

**Two layers:**

1. **Hetzner Cloud API coverage (main track):** add and harden CRDs/reconcilers for the resource types teams use (servers, networks, volumes, load balancers, firewalls, placement groups, …). Users adopt **any subset**. Progress: [docs/hcloud-api-coverage.md](hcloud-api-coverage.md).
2. **Composition recipes (optional track):** examples that wire primitives together. Includes k3s/hetzner-k3s-oriented docs under `docs/k3s-*` and `config/samples/complex/k3s-*`.

## Main track: Hetzner Cloud API coverage

Tracked on GitHub milestone **[HKIC: Hetzner Cloud CRD coverage](https://github.com/armanfeyzi/hcloud-operator/milestone/1)** (issues #1–#9).

### Shipped (v1alpha1)

- [x] `HCloudServer` — create / delete / adopt-by-name, type changes, `networkRef`
- [x] `HCloudVolume` — lifecycle, attach via `serverRef`
- [x] `HCloudLoadBalancer` — lifecycle, target sync via `serverSelector`
- [x] `HCloudNetwork` — private networks, Cloud subnets
- [x] `HCloudFirewall` — rules, attach via `serverRefs` / label selector
- [x] `HCloudPlacementGroup` — spread/cluster groups; `HCloudServer.spec.placementGroupRef`

### In progress / planned

- [x] **API coverage matrix** — [docs/hcloud-api-coverage.md](hcloud-api-coverage.md) (#1)
- [x] **`HCloudPlacementGroup`** + `HCloudServer.spec.placementGroupRef` (#2, #3)
- [ ] **`HCloudPrimaryIP`** (#4)
- [ ] **`HCloudFloatingIP`** (optional) (#5)
- [ ] **`HCloudCertificate`** + LB TLS integration (#6)
- [ ] **Volume snapshots** (`HCloudVolumeSnapshot` or spec model) (#7)
- [ ] **`HCloudSSHKey`** (optional / low priority) (#8)
- [x] **Harden existing CRDs** — LB listeners/health, `upgradeDisk` on server, volume resize (#9)

## Phase 1: MVP — **Complete**
- [x] Project scaffolding
- [x] `HCloudServer` CRD and reconciler (create / delete / adopt-by-name)
- [x] Status sync (IPs, state, conditions)
- [x] Finalizer-based cleanup

## Phase 2: Compute & storage — **Mostly complete**
- [x] Server vertical scaling (`spec.serverType` changes via power off → `change_type` → power on; `status.appliedServerType`)
- [x] Optional **`upgradeDisk`** on type change (`HCloudServer.spec.upgradeDisk`)
- [x] `HCloudVolume` CRD and reconciler
- [x] Volume attach / detach via `serverRef` and drift correction
- [x] `HCloudLoadBalancer` CRD with `serverSelector` and target sync
- [x] Load balancer reconciler watches `HCloudServer` so label / status changes re-sync targets without waiting for periodic requeue
- [x] **`Watches` on `HCloudServer` from `HCloudVolume`** — re-attach as soon as `status.serverID` appears
- [x] **Conflict-safe status updates** — retry-on-conflict for controller status writes
- [ ] **Observability** — Kubernetes `Events`, Prometheus metrics, richer conditions (#11)

## Phase 3: Networking — **Complete**
- [x] `HCloudNetwork` CRD and reconciler
- [x] Attach Cloud Servers to `HCloudNetwork` (`HCloudServer.spec.networkRef`)
- [x] `HCloudFirewall` CRD and reconciler

## Phase 4: GitOps & platform engineering
- [ ] **Helm chart** (image, token secret, optional feature flags) — primary install path alongside kustomize (#12)
- [ ] Argo CD Application examples / patterns (#13)
- [ ] Optional **CI E2E** against real Hetzner when `HCLOUD_TOKEN` is present in repo secrets (#14)
- [ ] Multi-cluster / multi-token patterns (documentation first) (#15)
- [ ] Higher-level abstractions with [kro](https://github.com/kubernetes-sigs/kro) or Crossplane Compositions (recipes + docs) (#16)

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

| Milestone | Focus |
|---|---|
| **HKIC: Hetzner Cloud CRD coverage** | Main ACK track — new CRDs, hardening (#1–#9) |
| **HKIC: Operator & GitOps platform** | Helm, Argo, observability, CI (#11–#16) |
| **HKIC: Parity & verification** | Optional k3s recipe verification (#10) |
