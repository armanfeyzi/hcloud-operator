# Project Roadmap

The vision of the **Hetzner Kubernetes Infrastructure Controller (HKIC)** is to provide a comprehensive, Kubernetes-native infrastructure abstraction for Hetzner Cloud.

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
- [x] `HCloudNetwork` CRD (private networks: `ipRange`, optional `networkZones` for Cloud subnets, labels)
- [ ] Attach Cloud Servers to `HCloudNetwork` (API: attach server to network — new field on `HCloudServer` or separate CRD)
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
- [ ] Keep **`.idea/project-idea.md`** aligned with README / roadmap when major features land
