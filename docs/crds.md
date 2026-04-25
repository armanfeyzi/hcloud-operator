# Custom Resource Definitions (CRDs)

## `HCloudServer`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a single Hetzner Cloud virtual server.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `serverType` | string | Yes | Hetzner server type (e.g. `cx21`, `cpx31`). Can be updated after creation; the operator stops the VM, calls Hetzner `change_type`, then starts it again. |
| `upgradeDisk` | bool | No | When true, disk is upgraded during a `serverType` change (Hetzner `change_type` with `upgrade_disk`). |
| `image` | string | Yes | OS image (e.g. `ubuntu-22.04`, `debian-11`) |
| `location` | string | Yes | Datacenter location (`fsn1`, `nbg1`, `hel1`, `ash`, `hil`) |
| `labels` | map[string]string | No | Cloud resource labels |
| `sshKeys` | []string | No | List of SSH key names/IDs to inject at creation |
| `userData` | string | No | Cloud-init configuration (applied at **server creation** in Hetzner; see [k3s sample](k3s-on-hcloud.md)) |
| `networkRef.name` | string | No | Name of target `HCloudNetwork` to attach this server to |
| `placementGroupRef.name` | string | No | Name of target `HCloudPlacementGroup`; applied at **server creation** only |

### Status

| Field | Type | Description |
|---|---|---|
| `serverID` | int64 | The internal Hetzner Cloud ID of the created server |
| `state` | string | Current state (e.g. `running`, `off`, `initializing`) |
| `appliedServerType` | string | Last server type the controller fully reconciled (spec matches Hetzner and the server was `running`). Empty or stale while a type change is in progress. |
| `publicIPv4` | string | The allocated public IPv4 address |
| `publicIPv6` | string | The allocated public IPv6 address network |
| `appliedNetworkID` | int64 | Last private network ID managed via `spec.networkRef` (cleared when `networkRef` is unset) |
| `appliedPlacementGroupID` | int64 | Placement group ID used when the server was created |
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
| `size` | int | Yes | Size in GB (10-10240). Can be **increased** after creation; shrink is rejected by validation and the API. |
| `location` | string | Conditionally | Required when `serverRef` is not set |
| `serverRef.name` | string | No | Name of target `HCloudServer` to attach (cluster-scoped reference by name) |
| `format` | string | No | Filesystem type to create |
| `labels` | map[string]string | No | Cloud resource labels |

### Status

| Field | Type | Description |
|---|---|---|
| `volumeID` | int64 | Hetzner Cloud volume ID |
| `state` | string | Current state (e.g. `creating`, `available`) |
| `attachedServerID` | int64 | Hetzner server ID this volume is currently attached to |
| `appliedSize` | int | Last size in GB observed from Hetzner |
| `linuxDevice` | string | Linux device path exposed by Hetzner (for attached volumes) |
| `conditions` | []Condition | Status conditions (e.g. `Ready=True`) |

### Example

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudVolume
metadata:
  name: app-data
spec:
  size: 30
  format: ext4
  serverRef:
    name: app-server
```

## `HCloudNetwork`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a Hetzner Cloud private network and optional Cloud subnets per network zone.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `ipRange` | string | Yes | IPv4 CIDR for the network (e.g. `10.0.0.0/16`). Immutable after creation. |
| `networkZones` | []string | No | Hetzner network zones where a Cloud subnet is created (e.g. `eu-central`, `us-east`, `us-west`). Immutable after creation. |
| `exposeRoutesToVSwitch` | bool | No | Expose routes to an attached vSwitch when applicable. |
| `labels` | map[string]string | No | Cloud resource labels |

### Status

| Field | Type | Description |
|---|---|---|
| `networkID` | int64 | Hetzner Cloud network ID |
| `ipRange` | string | Observed CIDR |
| `subnetZones` | []string | Zones where a Cloud subnet exists |
| `conditions` | []Condition | Status conditions (e.g. `Ready=True`) |

### Example

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudNetwork
metadata:
  name: demo-private
spec:
  ipRange: 10.100.0.0/16
  networkZones:
    - eu-central
  labels:
    env: demo
```

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
| `services` | []Service | No | Listener/target port definitions with optional health checks (keyed by `listenPort`) |
| `labels` | map[string]string | No | Cloud resource labels |

### Service fields (`services[]`)

| Field | Type | Required | Description |
|---|---|---|---|
| `listenPort` | int | Yes | Public listener port (1-65535) |
| `destinationPort` | int | Yes | Target server port |
| `protocol` | string | Yes | `tcp`, `http`, or `https` |
| `proxyprotocol` | bool | No | Enable PROXY protocol |
| `healthCheck` | object | No | Active health check settings |

Health check fields: `protocol`, optional `port`, `intervalSeconds`, `timeoutSeconds`, `retries`, and optional `http` (`domain`, `path`, `response`, `statusCodes`, `tls`).

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

## `HCloudFirewall`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a Hetzner Cloud **Firewall** (rules + server / label attachments).

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `labels` | map[string]string | No | Labels on the Hetzner firewall resource |
| `rules` | []HCloudFirewallRule | No | Rule list; reconciled with the API’s *set rules* action (order is not used for drift detection) |
| `applyTo.labelSelector` | string | No | Hetzner Cloud server label selector (e.g. `env=prod`) |
| `applyTo.serverRefs` | []LocalObjectReference | No | `HCloudServer` names; `status.serverID` must be set before attachment |

### Rule fields (`HCloudFirewallRule`)

| Field | Type | Description |
|---|---|---|
| `direction` | string | `in` or `out` |
| `protocol` | string | `tcp`, `udp`, `icmp`, `esp`, or `gre` |
| `port` | string | Optional; e.g. `22` or `8080-8090` |
| `sourceIPs` | []string | Source CIDRs (e.g. `0.0.0.0/0`, `::/0`) |
| `destinationIPs` | []string | For outbound rules when used |
| `description` | string | Optional note in Hetzner |

### Status

| Field | Type | Description |
|---|---|---|
| `firewallID` | int64 | Hetzner firewall ID |
| `conditions` | []Condition | e.g. `Ready=True` |

### Example (SSH + k3s API from listed CIDRs)

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudFirewall
metadata:
  name: cluster-edge
spec:
  labels:
    role: k3s
  rules:
    - direction: in
      protocol: tcp
      port: "22"
      sourceIPs:
        - 203.0.113.0/24
    - direction: in
      protocol: tcp
      port: "6443"
      sourceIPs:
        - 203.0.113.0/24
  applyTo:
    serverRefs:
      - name: my-server
```

## `HCloudPlacementGroup`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a Hetzner Cloud placement group (spread or cluster anti-/co-location).

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | Yes | `spread` or `cluster`. Immutable after creation. |
| `labels` | map[string]string | No | Cloud resource labels |

### Status

| Field | Type | Description |
|---|---|---|
| `placementGroupID` | int64 | Hetzner placement group ID |
| `type` | string | Observed type in Hetzner |
| `conditions` | []Condition | e.g. `Ready=True` |

### Example

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudPlacementGroup
metadata:
  name: app-spread
spec:
  type: spread
  labels:
    app: web
```

Reference from a server at create time:

```yaml
spec:
  placementGroupRef:
    name: app-spread
```

## `HCloudPrimaryIP`
Group: `infra.hkc.io/v1alpha1`
Scope: `Cluster`

Manages a Hetzner Cloud primary IP (IPv4 or IPv6) with optional assignment to an `HCloudServer`.

Primary IPs replace the default public addresses on servers when assigned. The referenced server must exist in the same datacenter family as the primary IP's datacenter.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | Yes | `ipv4` or `ipv6`. Immutable after creation. |
| `datacenter` | string | Yes | Hetzner datacenter (e.g. `fsn1-dc14`). Immutable after creation. |
| `serverRef.name` | string | No | `HCloudServer` to assign this primary IP to |
| `autoDelete` | bool | No | Delete the primary IP when the assignee is deleted |
| `dnsPtr` | string | No | Reverse DNS entry for the allocated address |
| `labels` | map[string]string | No | Cloud resource labels |

### Status

| Field | Type | Description |
|---|---|---|
| `primaryIPID` | int64 | Hetzner primary IP ID |
| `ip` | string | Allocated address observed in Hetzner |
| `datacenter` | string | Observed datacenter |
| `appliedAssigneeID` | int64 | Hetzner server ID this primary IP is assigned to |
| `conditions` | []Condition | e.g. `Ready=True` |

### Example

```yaml
apiVersion: infra.hkc.io/v1alpha1
kind: HCloudPrimaryIP
metadata:
  name: app-ipv4
spec:
  type: ipv4
  datacenter: fsn1-dc14
  autoDelete: true
  dnsPtr: app.example.com
  serverRef:
    name: web-node-1
```
