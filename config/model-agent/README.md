# P2P Model Distribution

This directory contains Kubernetes resources for P2P-enabled model distribution in OME.

## Overview

P2P model distribution uses BitTorrent protocol to efficiently distribute large model files across cluster nodes. Instead of each node downloading from HuggingFace simultaneously (which causes rate limiting), the first node downloads and seeds to other nodes.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Cluster                                                    │
│                                                             │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐  │
│  │ Node 1  │◄──►│ Node 2  │◄──►│ Node 3  │◄──►│ Node N  │  │
│  │ (seed)  │    │ (leech) │    │ (leech) │    │ (leech) │  │
│  └────┬────┘    └─────────┘    └─────────┘    └─────────┘  │
│       │              ▲              ▲              ▲        │
│       │              └──────────────┴──────────────┘        │
│       │                   P2P via BitTorrent                │
│       ▼                                                     │
│  ┌─────────┐                                                │
│  │   HF    │  (only first node, coordinated via Lease)      │
│  └─────────┘                                                │
│                                                             │
│  Peer Discovery: Headless Service DNS                       │
│  Coordination: Kubernetes Lease                             │
│  Storage: hostPath (enables resume)                         │
└─────────────────────────────────────────────────────────────┘
```

## Components

### Headless Service (`headless-service.yaml`)

Enables DNS-based peer discovery. Pods can lookup other pods' IPs to establish BitTorrent connections.

### Model Agent DaemonSet (`model-agent-daemonset.yaml`)

Deploys the model-agent with P2P capabilities on each node. Includes:
- BitTorrent client (port 6881)
- Metainfo HTTP server (port 8081)
- Health checks and Prometheus metrics
- RBAC for leases, nodes, and model resources

## Installation

```bash
# Deploy the headless service
kubectl apply -f headless-service.yaml

# Deploy the DaemonSet with RBAC
kubectl apply -f model-agent-daemonset.yaml
```

## Configuration

CLI flags for P2P configuration (passed via `args` in the DaemonSet):

| Flag | Default | Description |
|------|---------|-------------|
| `--p2p-enabled` | `false` | Enable/disable P2P distribution |
| `--p2p-peers-service` | `ome-peers.ome.svc.cluster.local` | Headless service DNS for peer discovery |
| `--p2p-torrent-port` | `6881` | BitTorrent peer port |
| `--p2p-metainfo-port` | `8081` | HTTP port for metainfo sharing |
| `--p2p-max-download-rate` | `2147483648` | Max download rate (bytes/s, default 2 GB/s) |
| `--p2p-max-upload-rate` | `2147483648` | Max upload rate (bytes/s, default 2 GB/s) |
| `--p2p-enable-encryption` | `false` | Enable BitTorrent header obfuscation |
| `--p2p-require-encryption` | `false` | Require encryption for all peers |
| `--p2p-download-timeout` | `1h` | Timeout for P2P downloads |

Environment variables (from Kubernetes downward API, required):

| Variable | Description |
|----------|-------------|
| `NODE_NAME` | Node name from `spec.nodeName` |
| `POD_NAME` | Pod name from `metadata.name` |
| `POD_IP` | Pod IP from `status.podIP` |

## How It Works

1. **Pod starts**: Model-agent initializes P2P distributor
2. **Model request**: Scout detects new BaseModel/ClusterBaseModel
3. **Check local**: If model exists on hostPath, seed it
4. **Try P2P**: Query peers for model via metainfo HTTP
5. **Lease coordination**: If no peers have it, try to acquire lease
6. **HF download**: Lease holder downloads from HuggingFace
7. **Seed**: Downloaded model is seeded to other nodes
8. **Complete**: All nodes have the model

## Performance

| Nodes | HF Direct (parallel) | BitTorrent |
|-------|---------------------|------------|
| 1 | 20-40 min | 20-40 min (same) |
| 10 | Throttled/fails | ~5-8 min |
| 100 | Throttled/fails | ~5-10 min |

## Monitoring

Prometheus metrics are exposed on port 8080:

- `ome_p2p_download_total` - Total downloads by source (p2p, hf, local)
- `ome_p2p_download_duration_seconds` - Download duration histogram
- `ome_p2p_peers_discovered` - Number of peers found via DNS
- `ome_p2p_seeding_torrents` - Number of models being seeded
- `ome_p2p_bytes_uploaded_total` - Total bytes uploaded to peers
- `ome_p2p_leases_acquired_total` - Number of leases acquired

## Troubleshooting

### P2P not working

1. Check headless service exists:
   ```bash
   kubectl get svc ome-peers -n ome
   ```

2. Verify DNS resolution:
   ```bash
   kubectl exec -it <pod> -- nslookup ome-peers.ome.svc.cluster.local
   ```

3. Check P2P ports are accessible:
   ```bash
   kubectl exec -it <pod> -- nc -zv <peer-ip> 6881
   ```

### Lease stuck

1. Check lease status:
   ```bash
   kubectl get leases -n ome -l ome.io/type=model-download
   ```

2. Delete stuck lease:
   ```bash
   kubectl delete lease ome-model-<hash> -n ome
   ```

### Rate limiting still occurring

1. Verify P2P is enabled:
   ```bash
   kubectl logs <pod> | grep "P2P"
   ```

2. Check only one node is downloading:
   ```bash
   kubectl get leases -n ome -o yaml
   ```
