# Project Roadmap

The vision of the **Hetzner Kubernetes Infrastructure Controller (HKIC)** is to provide a comprehensive, Kubernetes-native infrastructure abstraction for Hetzner Cloud.

**Same page — two layers:**

1. **Broad Hetzner Cloud coverage:** HKIC should keep gaining the ability to **create, configure, update, and delete** the Hetzner Cloud resource types people actually use (servers, networks, volumes, load balancers, firewalls, placement/labels, and so on), with users free to adopt **any subset** of controllers.
2. **Kubernetes on Hetzner as a composition:** If someone wants a **Kubernetes cluster on Hetzner Cloud** comparable to what [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) provides, they should be able to achieve that **using HKIC’s controllers** (the same primitives, plus bootstrap and Day-2 docs/manifests), not only unrelated one-off VMs.

The milestone below is the **cluster-shaped** slice of (2); Phases 1–3 are the expanding **surface** for (1).

## Milestone: Parity with [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s)

**Goal:** Enable the **full lifecycle** of a Kubernetes cluster on Hetzner Cloud using HKIC — **deploy, run, reconfigure, modify, and delete** — with **coverage of the major Hetzner Cloud components and configs** needed for a production-style stack (comparable to what [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) provisions). Teams can use HKIC instead of the hetzner-k3s CLI when they want GitOps and Kubernetes-native infra objects.

**Composable architecture (non-negotiable):** each concern remains a **separate** CRD/reconciler where it makes sense (`HCloudServer`, `HCloudNetwork`, `HCloudFirewall`, …). Users may adopt **only the pieces they need**; “full cluster” is achieved by **combining** resources and optional higher-level recipes (Helm, [kro](https://github.com/kubernetes-sigs/kro), docs), not by hiding everything behind a single mandatory mega-API.

**Reference:** upstream documents HA masters, worker pools (including autoscaling min/max), private networking, firewalls, load balancing, k3s versioning, and Day-2 components (Hetzner [CCM](https://github.com/hetznercloud/hcloud-cloud-controller-manager), [CSI](https://github.com/hetznercloud/csi-driver), Cluster Autoscaler, System Upgrade Controller, optional CNI choices). HKIC should converge on that *surface area* at the Hetzner API layer; cluster-wide automation may be split across low-level CRDs and Phase 4 compositions.

### Parity-oriented tasks

- [x] **k3s sample + doc** — [docs/k3s-on-hcloud.md](k3s-on-hcloud.md) and `config/samples/k3s_single_node_*_v1alpha1.yaml` (single-node server bootstrap via `spec.userData`; multi-node / automation TBD)
- [x] **`HCloudFirewall`** CRD and reconciler (cluster firewall rules; attach via `serverRefs` and/or Hetzner `labelSelector`)
- [x] **Node bootstrap** — extended beyond static samples with join/token automation helpers: `hack/configure-k3s-join-agents.sh` + verification runbook/script (`hack/verify-k3s-join-cluster.sh`) while keeping composable CRDs
- [x] **Cluster shape abstraction** — documented composition recipe + runnable sample mapping **masters pool + worker pools + networking + firewall + LB** to existing CRDs: [docs/k3s-cluster-shape-recipe.md](k3s-cluster-shape-recipe.md), `config/samples/complex/k3s-cluster-shape/k3s_cluster_shape_v1alpha1.yaml`
- [ ] **Day-2 manifests** — versioned examples for Hetzner CCM, CSI, Cluster Autoscaler, System Upgrade Controller (as apply/kustomize/Helm), matching what `hetzner-k3s create` installs (**CCM + CSI samples/doc added; autoscaler + system-upgrade pending**)
- [x] **Config mapping doc** — translate a representative `cluster.yaml` from hetzner-k3s into HKIC resources (field-by-field notes, gaps, extensions): [docs/hetzner-k3s-cluster-yaml-mapping.md](hetzner-k3s-cluster-yaml-mapping.md)
- [ ] **Parity checklist / E2E** — optional CI or runbook: bring up a cluster via HKIC and verify the same operational properties we care about (private connectivity, API availability, CSI storage class, CCM LB, etc.)

## Phase 1: MVP — **Complete**
- [x] Project scaffolding
- [x] `HCloudServer` CRD and reconciler (create / delete / adopt-by-name)
- [x] Status sync (IPs, state, conditions)
- [x] Finalizer-based cleanup

## Phase 2: Compute & storage — **Mostly complete**
- [x] Server vertical scaling (`spec.serverType` changes via power off → `change_type` → power on; `status.appliedServerType`)
- [x] Optional **`upgradeDisk`** on type change (not yet exposed on `HCloudServer` spec — backlog)
- [x] `HCloudVolume` CRD and reconciler
- [x] Volume attach / detach via `serverRef` and drift correction
- [x] `HCloudLoadBalancer` CRD with `serverSelector` and target sync
- [x] Load balancer reconciler watches `HCloudServer` so label / status changes re-sync targets without waiting for periodic requeue
- [x] **`Watches` on `HCloudServer` from `HCloudVolume`** — re-attach as soon as `status.serverID` appears (volume now requeues promptly on server changes)
- [x] **Conflict-safe status updates** — reduced `object has been modified` noise using retry-on-conflict for controller status writes
- [ ] **Observability** — Kubernetes `Events`, Prometheus metrics, richer conditions

## Phase 3: Networking & clustering — **In progress**
- [x] `HCloudNetwork` CRD and reconciler (private networks: `ipRange`, optional `networkZones` for Cloud subnets, labels; adopt-by-name; finalizer cleanup)
- [x] Attach Cloud Servers to `HCloudNetwork` (`HCloudServer.spec.networkRef`)
- [x] `HCloudFirewall` CRD and reconciler
- [x] Cloud-init / node bootstrap templates (e.g. k3s join) built on networks + servers

## Phase 4: GitOps & platform engineering
- [ ] **Helm chart** (image, token secret, optional feature flags) — primary install path alongside kustomize
- [ ] Argo CD Application examples / patterns
- [ ] Optional **CI E2E** against real Hetzner when `HCLOUD_TOKEN` is present in repo secrets
- [ ] Multi-cluster / multi-token patterns (documentation first)
- [ ] Higher-level abstractions with [kro](https://github.com/kubernetes-sigs/kro) or Crossplane Compositions (recipes + docs)

## Hygiene & contributor experience
- [x] Envtest isolation: drain cluster CRs + reset shared `FakeClient` between Ginkgo specs (random order safe)
- [x] Keep **`.idea/project-idea.md`** and **`.idea/AGENT_*.md`** aligned with README / roadmap when major features land (north star: hetzner-k3s parity)
