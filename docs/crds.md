# Custom Resource Definitions (CRDs)

## `HCloudServer`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a single Hetzner Cloud virtual server.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `serverType` | string | Yes | Hetzner server type (e.g. `cx21`, `cpx31`) |
| `image` | string | Yes | OS image (e.g. `ubuntu-22.04`, `debian-11`) |
| `location` | string | Yes | Datacenter location (`fsn1`, `nbg1`, `hel1`, `ash`, `hil`) |
| `labels` | map[string]string | No | Cloud resource labels |
| `sshKeys` | []string | No | List of SSH key names/IDs to inject at creation |
| `userData` | string | No | Cloud-init configuration |

### Status

| Field | Type | Description |
|---|---|---|
| `serverID` | int64 | The internal Hetzner Cloud ID of the created server |
| `state` | string | Current state (e.g. `running`, `off`, `initializing`) |
| `publicIPv4` | string | The allocated public IPv4 address |
| `publicIPv6` | string | The allocated public IPv6 address network |
| `conditions` | []Condition | Status conditions (e.g. `Ready=True`) |

### Example

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudServer
metadata:
  name: web-node-1
spec:
  serverType: cx21
  image: ubuntu-22.04
  location: fsn1
```

## `HCloudVolume`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a single Hetzner Cloud volume and optional attachment to a server.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `size` | int | Yes | Size in GB (10-10240) |
| `location` | string | Conditionally | Required when `serverRef` is not set |
| `serverRef.name` | string | No | Name of target `HCloudServer` to attach |
| `format` | string | No | Filesystem type to create |
| `labels` | map[string]string | No | Cloud resource labels |

## `HCloudLoadBalancer`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a Hetzner Cloud Load Balancer and keeps its targets in sync with matching `HCloudServer` resources.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `loadBalancerType` | string | Yes | Hetzner LB type (e.g. `lb11`) |
| `location` | string | No | Datacenter location (mutually exclusive with `networkZone`) |
| `networkZone` | string | No | Network zone (mutually exclusive with `location`) |
| `algorithm` | string | No | Balancing algorithm (`round_robin` or `least_connections`) |
| `serverSelector` | LabelSelector | No | Selects `HCloudServer` objects by Kubernetes labels |
| `labels` | map[string]string | No | Cloud resource labels |

### Example

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudLoadBalancer
metadata:
  name: public-web
spec:
  loadBalancerType: lb11
  location: fsn1
  algorithm: round_robin
  serverSelector:
    matchLabels:
      app: web
```
