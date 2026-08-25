# Stackryze Edge Agent

A lightweight monitoring agent you run on **your own compute** (a VM, container,
or Kubernetes) in the regions you care about. It measures DNS resolution latency
and target reachability for your Stackryze zones from that vantage point, then
pushes only **aggregated metrics** to the Stackryze control plane. The analysis
UI lives in the Stackryze dashboard (dns.stackryze.com).

```
your region (VM / pod)
        │
   stackryze-edge  ──resolve + dial──►  your zones' records
        │  aggregates locally (latency p50/p95, success %, targets up/down)
        ▼  Bearer token
   api.stackryze.com/api/edge/metrics
        │
        ▼
   Stackryze dashboard → Edge monitoring
```

No raw queries ever leave your infrastructure — only per-window aggregates.
Runs entirely on your compute, so it never touches Stackryze's serving path.

## Configuration (env)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `STACKRYZE_API_TOKEN` | yes | — | Token with **write** scope (Settings → API tokens) |
| `STACKRYZE_API_URL` | no | `https://api.stackryze.com/api` | API base |
| `STACKRYZE_EDGE_REGION` | no | `default` | Vantage-point label shown in the UI (e.g. `mumbai`) |
| `STACKRYZE_EDGE_AGENT_ID` | no | `<region>-<hostname>` | Stable agent identifier |
| `STACKRYZE_EDGE_INTERVAL` | no | `60` | Seconds between cycles (min 15) |
| `STACKRYZE_EDGE_ZONES` | no | (all your zones) | Comma list to monitor; empty = auto-discover |
| `STACKRYZE_EDGE_RESOLVERS` | no | `8.8.8.8,1.1.1.1,9.9.9.9` | Resolver IPs to test against |

## Run

```bash
STACKRYZE_API_TOKEN=sk_dns_xxx \
STACKRYZE_EDGE_REGION=mumbai \
./stackryze-edge
```

### Docker

```bash
docker run -d --name stackryze-edge \
  -e STACKRYZE_API_TOKEN=sk_dns_xxx \
  -e STACKRYZE_EDGE_REGION=mumbai \
  ghcr.io/stackryze/stackryze-edge:latest
```

### Kubernetes (one agent per region)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stackryze-edge
spec:
  replicas: 1
  selector:
    matchLabels: { app: stackryze-edge }
  template:
    metadata:
      labels: { app: stackryze-edge }
    spec:
      containers:
        - name: agent
          image: ghcr.io/stackryze/stackryze-edge:latest
          env:
            - name: STACKRYZE_EDGE_REGION
              value: mumbai
            - name: STACKRYZE_API_TOKEN
              valueFrom:
                secretKeyRef: { name: stackryze-dns, key: token }
```

## What it measures

Per zone, per cycle:
- **Resolution latency** (p50/p95) resolving the apex across the configured resolvers.
- **Success rate** of those lookups.
- **Target reachability** — dials each resolved IP on `:443` (up/down counts).

## Build

Requires Go 1.22+. Pure standard library, no external dependencies.

```bash
go build -o stackryze-edge .
```
