# HKIC Architecture

## Overview
Hetzner Kubernetes Infrastructure Controller (HKIC) is a Kubernetes-native operator that provisions and manages Hetzner Cloud infrastructure using the Operator pattern.

## Components

1. **Custom Resource Definitions (CRDs)**
   - The API layer defined in Kubernetes.
   - Example: `HCloudServer` defines the desired state of a Hetzner Cloud VM.

2. **Controller (Reconciler)**
   - Runs as a pod in the cluster.
   - Watches for changes to `HCloudServer` resources.
   - Compares desired state (CRD spec) against actual state (Hetzner Cloud).

3. **Hetzner Cloud Client Wrapper**
   - Thin wrapper around the official `hcloud-go` SDK.
   - Adds idempotency and Kubernetes-specific error handling.

## Reconciliation Loop (HCloudServer)

```mermaid
graph TD
    A[CRD Changed/Created] --> B{Deletion Timestamp set?}
    B -- Yes --> C[Run Finalizer: Delete from Hetzner]
    C --> D[Remove Finalizer]
    B -- No --> E{Has Finalizer?}
    E -- No --> F[Add Finalizer]
    F --> G[Requeue]
    E -- Yes --> H[Fetch Server from Hetzner]
    H --> I{Server Exists?}
    I -- No --> J[Create Server via API]
    J --> K[Update Status with Server ID]
    I -- Yes --> L{Spec == Actual?}
    L -- No --> M[Update Server]
    L -- Yes --> N[Update Status IPs & State]
```
