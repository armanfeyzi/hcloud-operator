# Project Roadmap

The vision of the **Hetzner Kubernetes Infrastructure Controller (HKIC)** is to provide a comprehensive, Kubernetes-native infrastructure abstraction for Hetzner Cloud.

## Phase 1: MVP (Current)
- [x] Project Scaffolding
- [x] `HCloudServer` CRD definition
- [x] Reconciler for creating/deleting servers
- [x] Basic status synchronization (IPs, state)

## Phase 2: Compute Expansion
- [ ] Support for server scale up / down
- [ ] `HCloudVolume` CRD (Block Storage)
- [ ] `HCloudLoadBalancer` CRD
- [ ] Volume attachment to Servers

## Phase 3: Networking & Clustering
- [ ] `HCloudNetwork` CRD (Private Networks)
- [ ] Cloud-init node bootstrap templates (e.g. k3s auto-join)
- [ ] Firewalls support

## Phase 4: GitOps & Advanced
- [ ] ArgoCD templates / patterns
- [ ] Helm Chart packaging
- [ ] Multi-cluster management support
