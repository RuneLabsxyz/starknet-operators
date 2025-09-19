# StarknetRPC Monitoring with PodMonitor

This document describes how to enable and configure Prometheus monitoring for StarknetRPC resources using PodMonitor.

## Overview

The StarknetRPC operator supports automatic creation of PodMonitor resources for Prometheus-based monitoring. When enabled, the operator will create a PodMonitor that allows Prometheus to scrape metrics from the StarknetRPC pods.

## Prerequisites

- Prometheus Operator must be installed in your cluster
- The PodMonitor CRD must be available

## Configuration

### Basic Configuration

By default, PodMonitor creation is **disabled**. To enable monitoring, add the `podMonitor` field to your StarknetRPC spec:

```yaml
apiVersion: pathfinder.runelabs.xyz/v1alpha1
kind: StarknetRPC
metadata:
  name: starknet-mainnet
spec:
  network: mainnet
  
  # Enable PodMonitor creation
  podMonitor:
    enabled: true
  
  # ... other configuration
```

### Advanced Configuration with Custom Labels

You can add custom labels to the PodMonitor resource. These labels can be used by Prometheus to discover and select the PodMonitor:

```yaml
apiVersion: pathfinder.runelabs.xyz/v1alpha1
kind: StarknetRPC
metadata:
  name: starknet-mainnet
spec:
  network: mainnet
  
  podMonitor:
    enabled: true
    labels:
      # Custom labels for Prometheus discovery
      prometheus: kube-prometheus
      team: platform
      environment: production
      monitoring-tier: critical
  
  # ... other configuration
```

## Default Behavior

When the `podMonitor` field is not specified or when `enabled: false`, no PodMonitor will be created:

```yaml
apiVersion: pathfinder.runelabs.xyz/v1alpha1
kind: StarknetRPC
metadata:
  name: starknet-mainnet
spec:
  network: mainnet
  # podMonitor field not specified - monitoring disabled by default
  # ... other configuration
```

Or explicitly disabled:

```yaml
apiVersion: pathfinder.runelabs.xyz/v1alpha1
kind: StarknetRPC
metadata:
  name: starknet-mainnet
spec:
  network: mainnet
  podMonitor:
    enabled: false  # Explicitly disabled
  # ... other configuration
```

## PodMonitor Details

When enabled, the operator creates a PodMonitor with the following characteristics:

### Metrics Endpoint
- **Port**: `monitoring` (port 9000)
- **Path**: `/metrics`

### Default Labels

The PodMonitor will have these default labels:

| Label | Value |
|-------|-------|
| `rpc.runelabs.xyz/type` | `starknet` |
| `rpc.runelabs.xyz/name` | `<resource-name>` |
| `runelabs.xyz/network` | `<network-value>` |
| `app.kubernetes.io/name` | `starknet-rpc` |
| `app.kubernetes.io/instance` | `<resource-name>` |
| `app.kubernetes.io/managed-by` | `starknet-operator` |

Custom labels specified in `podMonitor.labels` will be added to these defaults.

### Pod Selection

The PodMonitor selects pods using the label:
- `rpc.runelabs.xyz/name: <resource-name>`

### Target Labels

The following labels from the pod will be attached to the scraped metrics:
- `runelabs.xyz/network`
- `rpc.runelabs.xyz/name`

## Lifecycle Management

### Creation
The PodMonitor is created automatically when:
1. `podMonitor.enabled` is set to `true`
2. The StarknetRPC pod exists

### Updates
If you modify the `podMonitor` configuration, the operator will automatically update the existing PodMonitor to match the new configuration.

### Deletion
The PodMonitor will be automatically deleted when:
- The StarknetRPC resource is deleted (due to owner references)
- The `podMonitor.enabled` is changed from `true` to `false`
- The `podMonitor` field is removed from the spec

## Example Use Cases

### 1. Development Environment
Simple monitoring without custom labels:

```yaml
podMonitor:
  enabled: true
```

### 2. Production Environment with Service Discovery
Using labels for Prometheus service discovery:

```yaml
podMonitor:
  enabled: true
  labels:
    prometheus: production-prometheus
    alerting: enabled
    sla: high
```

### 3. Multi-tenant Cluster
Segregating monitoring by team:

```yaml
podMonitor:
  enabled: true
  labels:
    team: blockchain
    cost-center: engineering
    prometheus-instance: team-blockchain
```

## Troubleshooting

### PodMonitor Not Created

1. **Check if monitoring is enabled:**
   ```bash
   kubectl get starknetrpc <name> -o jsonpath='{.spec.podMonitor.enabled}'
   ```

2. **Verify the pod exists:**
   ```bash
   kubectl get pods -l rpc.runelabs.xyz/name=<resource-name>
   ```

3. **Check if Prometheus Operator is installed:**
   ```bash
   kubectl get crd podmonitors.monitoring.coreos.com
   ```

### Metrics Not Being Scraped

1. **Verify PodMonitor exists:**
   ```bash
   kubectl get podmonitor <resource-name>-podmonitor
   ```

2. **Check PodMonitor labels match Prometheus serviceMonitorSelector:**
   ```bash
   kubectl get podmonitor <resource-name>-podmonitor -o yaml
   ```

3. **Verify pod is exposing metrics:**
   ```bash
   kubectl port-forward pod/<pod-name> 9000:9000
   curl http://localhost:9000/metrics
   ```

## Complete Example

See the [sample configurations](../config/samples/monitoring/) for complete examples:
- [starknetrpc_with_podmonitor.yaml](../config/samples/monitoring/starknetrpc_with_podmonitor.yaml) - Example with monitoring enabled
- [starknetrpc_without_podmonitor.yaml](../config/samples/monitoring/starknetrpc_without_podmonitor.yaml) - Example with monitoring disabled (default)