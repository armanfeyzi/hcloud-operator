# k3s Cluster Shape Recipe (Composable)

This recipe provides a "cluster-shaped" interface without introducing a mandatory mega-CRD.

It documents how to compose existing HKIC primitives into one reproducible stack:

- control-plane server(s)
- worker server(s)
- private network
- edge firewall policy
- optional Hetzner load balancer

## Why this approach

HKIC stays composable by design. The cluster shape is expressed as a package of regular CRDs, so teams can:

- adopt the full stack
- remove pieces they do not need
- evolve each concern independently

## Reference sample

Use this sample as the baseline composition:

- `config/samples/complex/k3s-cluster-shape/k3s_cluster_shape_v1alpha1.yaml`

It includes:

1. one `HCloudNetwork`
2. one control-plane `HCloudServer`
3. two worker `HCloudServer` objects
4. one `HCloudFirewall` attached by label selector
5. one optional `HCloudLoadBalancer` targeting worker labels

## Apply flow

```bash
kubectl apply -f config/samples/complex/k3s-cluster-shape/k3s_cluster_shape_v1alpha1.yaml
hack/configure-k3s-join-agents.sh k3s-shape-server
hack/verify-k3s-join-cluster.sh k3s-shape-server
```

Optional Day-2 verification (after CCM+CSI install):

```bash
hack/verify-k3s-join-cluster.sh k3s-shape-server 3 ~/.ssh/id_ed25519 600 true
```

## Input knobs (edit before apply)

- SSH key placeholder: `REPLACE_WITH_HETZNER_SSH_KEY_NAME`
- datacenter: `spec.location` on each server
- VM sizing: `spec.serverType`
- private CIDR: `HCloudNetwork.spec.ipRange`
- firewall source ranges for SSH/API

## Intent mapping

This recipe intentionally represents the same high-level shape users expect from a `cluster.yaml`:

- server pool roles are modeled with labels (`role=k3s-server`, `role=k3s-agent`)
- pool size is modeled by object count (today)
- infra security is encoded with `HCloudFirewall`
- network topology is encoded with `HCloudNetwork`

For field-level mapping details, see:

- `docs/hetzner-k3s-cluster-yaml-mapping.md`

## Future evolution

This recipe is the bridge to a higher-level abstraction (Helm or kro) that renders the same CRDs from a single values object. Until that lands, this sample is the recommended cluster-shape baseline.
