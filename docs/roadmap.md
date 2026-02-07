# Project Roadmap

The vision of the **Hetzner Kubernetes Infrastructure Controller (HKIC)** is to provide a comprehensive, Kubernetes-native infrastructure abstraction for Hetzner Cloud.

## Milestone: Parity with [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s)

**Goal:** Reproduce the same *class* of production-ready **k3s** clusters on Hetzner Cloud using HKIC (CRDs + controllers + GitOps), so teams can **replace the [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) CLI provisioning path** when they want Kubernetes-native infrastructure objects instead of a single-machine bootstrap tool.

**Reference:** upstream documents HA masters, worker pools (including autoscaling min/max), private networking, firewalls, load balancing, k3s versioning, and Day-2 components (Hetzner [CCM](https://github.com/hetznercloud/hcloud-cloud-controller-manager), [CSI](https://github.com/hetznercloud/csi-driver), Cluster Autoscaler, System Upgrade Controller, optional CNI choices). HKIC should converge on that *surface area*; implementation may split low-level CRDs from higher-level compositions (see Phase 4).

### Parity-oriented tasks

- [ ] **`HCloudFirewall`** CRD and reconciler (cluster firewall rules aligned with how hetzner-k3s locks down SSH/API/worker traffic)
- [ ] **Node bootstrap** — cloud-init (or equivalent) templates to install k3s server/agent roles, wired to `HCloudServer` / network / SSH keys
- [ ] **Cluster shape abstraction** — either a dedicated “cluster” API (`HCloudK3sCluster`-style) or a documented [kro](https://github.com/kubernetes-sigs/kro) / Helm recipe that maps **masters pool + worker pools + networking + LB** to existing CRDs
- [ ] **Day-2 manifests** — versioned examples for Hetzner CCM, CSI, Cluster Autoscaler, System Upgrade Controller (as apply/kustomize/Helm), matching what `hetzner-k3s create` installs
- [ ] **Config mapping doc** — translate a representative `cluster.yaml` from hetzner-k3s into HKIC resources (field-by-field notes, gaps, extensions)
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
- [ ] **`Watches` on `HCloudServer` from `HCloudVolume`** — re-attach as soon as `status.serverID` appears (today: volume periodic requeue)
- [ ] **Conflict-safe status updates** — reduce `object has been modified` noise (`Patch` / retry on conflict) for hot paths
- [ ] **Observability** — Kubernetes `Events`, Prometheus metrics, richer conditions

## Phase 3: Networking & clustering — **In progress**
- [x] `HCloudNetwork` CRD and reconciler (private networks: `ipRange`, optional `networkZones` for Cloud subnets, labels; adopt-by-name; finalizer cleanup)
- [x] Attach Cloud Servers to `HCloudNetwork` (`HCloudServer.spec.networkRef`)
- [ ] `HCloudFirewall` CRD and reconciler
- [ ] Cloud-init / node bootstrap templates (e.g. k3s join) built on networks + servers

## Phase 4: GitOps & platform engineering
- [ ] **Helm chart** (image, token secret, optional feature flags) — primary install path alongside kustomize
- [ ] Argo CD Application examples / patterns
- [ ] Optional **CI E2E** against real Hetzner when `HCLOUD_TOKEN` is present in repo secrets
- [ ] Multi-cluster / multi-token patterns (documentation first)
- [ ] Higher-level abstractions with [kro](https://github.com/kubernetes-sigs/kro) or Crossplane Compositions (recipes + docs)

## Hygiene & contributor experience
- [x] Envtest isolation: drain cluster CRs + reset shared `FakeClient` between Ginkgo specs (random order safe)
- [x] Keep **`.idea/project-idea.md`** and **`.idea/AGENT_*.md`** aligned with README / roadmap when major features land (north star: hetzner-k3s parity)
