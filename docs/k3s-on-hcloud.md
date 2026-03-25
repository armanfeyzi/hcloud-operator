# k3s on Hetzner Cloud with HKIC

This guide shows how to provision a **single-node k3s** cluster using HKIC primitives: an `HCloudNetwork`, an `HCloudServer` with **cloud-init** (`spec.userData`), and your existing SSH keys in Hetzner Cloud. The pattern matches how tools like [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) think about networking and nodes—here the **data plane** is plain CRDs plus `userData`, not a separate CLI.

## What you get

- One private network and one server attached to it (same idea as “private networking” in hetzner-k3s).
- k3s installed on first boot via cloud-init, with:
  - **Node / advertise address** on the private NIC (`eth1` on Ubuntu images when a network is attached).
  - **TLS SAN** for the server’s **public** IPv4 (from Hetzner [instance metadata](http://169.254.169.254/hetzner/v1/metadata)) so you can run `kubectl` from your laptop over the public IP.
  - **Traefik disabled** (`--disable traefik`) so you can choose your own ingress later.

## Prerequisites

- HKIC CRDs installed (`make install` or `kubectl apply -f config/crd/bases/`).
- Operator running with a valid `HCLOUD_TOKEN` (local `make run` or in-cluster deployment). See [Development](development.md).
- At least one **SSH key** registered in the Hetzner Cloud console (Security → SSH keys). You need the key **name** as shown in the console (not the public key string).
- `kubectl` on your machine, pointed at the cluster where HKIC runs (management cluster).

## Limitations (read before production)

| Topic | Detail |
|--------|--------|
| **`userData`** | Applied by Hetzner **when the server is created**. Changing `spec.userData` on an existing `HCloudServer` does not re-run cloud-init. To change bootstrap, replace the server (new object or delete/recreate). |
| **Image / location** | `spec.image` and `spec.location` are **immutable** after creation (API validation). |
| **`HCloudFirewall`** | Available — see `HCloudFirewall` CRD and `config/samples/simple/hcloud-firewall/hcloud_firewall_v1alpha1.yaml`. Attach by `applyTo.serverRefs` (after `HCloudServer.status.serverID` exists) and/or `applyTo.labelSelector`. |
| **HA / multi-node** | The default quickstart is **one server**. For multi-node bootstrap, use `config/samples/complex/k3s-multi-node-join/k3s_multi_node_join_v1alpha1.yaml` and run `hack/configure-k3s-join-agents.sh k3s-join-server` after the server is up (two-phase join flow with auto-discovery of agent CRs by label selector). |
| **Not hetzner-k3s** | CCM, CSI, Cluster Autoscaler, etc. are **not** installed by this sample—only k3s. See [Roadmap](roadmap.md) for Day-2 parity work. |

## Quick start

1. **Copy the sample** and set your SSH key name and sizes/locations as needed:

   ```bash
   cp config/samples/complex/k3s-single-node-private-net/k3s_single_node_private_net_v1alpha1.yaml /tmp/k3s-node.yaml
   # Edit: spec.sshKeys on the HCloudServer
   ```

2. **Apply**:

   ```bash
   kubectl apply -f /tmp/k3s-node.yaml
   ```

3. **Wait for the server** (watch `kubectl get hcs -w` until `status.state` is `running` and `status.publicIPv4` is set). Cloud-init may need another minute to finish installing k3s.

4. **SSH in** (user `root` on Ubuntu images):

   ```bash
   ssh root@<status.publicIPv4>
   ```

   On the server, confirm k3s:

   ```bash
   sudo k3s kubectl get nodes
   ```

5. **Use kubeconfig from your laptop**:

   ```bash
   scp root@<status.publicIPv4>:/etc/rancher/k3s/k3s.yaml ./k3s.yaml
   ```

   The file points at `https://127.0.0.1:6443` by default. Point it at the public IP and use it:

   ```bash
   export KUBECONFIG=$PWD/k3s.yaml
   # Replace the server URL (example with sed — adjust if your file differs)
   sed -i 's|https://127.0.0.1:6443|https://<status.publicIPv4>:6443|' k3s.yaml
   kubectl get nodes
   ```

   Because the install script added `--tls-san` for the public IP, the API certificate should validate for that address.

## Secure kubeconfig handling (recommended)

Avoid printing kubeconfig in controller logs. A safer pattern is:

1. Fetch kubeconfig from the server over SSH.
2. Store it as a Kubernetes Secret with restricted RBAC access.
3. Pull it locally only when needed.

This repo includes a helper script:

```bash
chmod +x hack/export-k3s-kubeconfig-to-secret.sh
hack/export-k3s-kubeconfig-to-secret.sh <server-public-ip> <secret-name> [namespace] [ssh-key-path]
```

Example:

```bash
hack/export-k3s-kubeconfig-to-secret.sh <status.publicIPv4> k3s-join-kubeconfig kube-public ~/.ssh/id_ed25519
```

Retrieve later:

```bash
kubectl get secret -n kube-public k3s-join-kubeconfig -o jsonpath='{.data.config}' | base64 -d > ./k3s-from-secret.yaml
KUBECONFIG=./k3s-from-secret.yaml kubectl get nodes
```

## Troubleshooting

- **Private IP / `eth1`:** On some images the private NIC may not be `eth1`. Check with `ip -4 addr` over SSH. If installs fail, adjust `userData` (`--flannel-iface`, `--node-ip`, `--advertise-address`) to match your interface and address.
- **Metadata:** Public IP is read from `http://169.254.169.254/hetzner/v1/metadata/public-ipv4` (see [hcloud-go metadata client](https://github.com/hetznercloud/hcloud-go/blob/main/hcloud/metadata/client.go)).
- **Cloud-init logs:** `/var/log/cloud-init-output.log` on the server.

## Cleanup

Delete the Kubernetes objects (finalizers remove Hetzner resources):

```bash
kubectl delete -f /tmp/k3s-node.yaml
```

Order may delete the server before the network; the controller should detach/delete as needed. If something sticks, check `kubectl get hcs,hcn` and events.

## Samples in this repo

| File | Purpose |
|------|---------|
| `config/samples/complex/k3s-single-node-private-net/k3s_single_node_private_net_v1alpha1.yaml` | Private network + single k3s server (recommended default). |
| `config/samples/complex/k3s-single-node-public-only/k3s_single_node_public_only_v1alpha1.yaml` | Single server, **no** `HCloudNetwork`—minimal smoke test (Flannel over public/default routing). |
| `config/samples/complex/k3s-multi-node-join/k3s_multi_node_join_v1alpha1.yaml` | Multi-node bootstrap template: one server plus agents, with post-provision join using `hack/configure-k3s-join-agents.sh k3s-join-server` (auto token fetch + agent label discovery). |
| `config/samples/complex/k3s-cluster-shape/k3s_cluster_shape_v1alpha1.yaml` | Cluster-shaped composition sample (network + firewall + control plane + worker pool + optional LB) using existing HKIC CRDs. |

Post-join verification helper:

```bash
hack/verify-k3s-join-cluster.sh k3s-join-server
```

Optional Day-2 smoke verification (after installing CCM + CSI):

```bash
hack/verify-k3s-join-cluster.sh k3s-join-server 3 ~/.ssh/id_ed25519 600 true
```

## Next steps

- Refine **multi-node** join automation (for example: token and server private IP wiring via a higher-level composition).
- Add **Day-2** manifests (Hetzner CCM, CSI) per [Roadmap](roadmap.md).
- Use **`HCloudFirewall`** for hetzner-k3s-style edge rules (SSH, API) instead of ad-hoc console work.
