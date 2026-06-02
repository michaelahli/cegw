# CEGW Helm Chart

Helm chart for deploying CEGW (Crypto Exchange Gateway) to Kubernetes.

## Installation

### Add Helm repository

```bash
helm repo add cegw https://michaelahli.github.io/cegw
helm repo update
```

### Install chart

```bash
helm install cegw cegw/cegw
```

### Install with custom values

```bash
helm install cegw cegw/cegw \
  --set replicaCount=3 \
  --set resources.limits.memory=512Mi
```

## Configuration

See [values.yaml](charts/cegw/values.yaml) for all configuration options.

### Key parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `ghcr.io/michaelahli/cegw` |
| `image.tag` | Image tag | `1.0.0` |
| `service.grpcPort` | gRPC port | `50051` |
| `service.httpPort` | HTTP port | `8080` |

## Uninstall

```bash
helm uninstall cegw
```
