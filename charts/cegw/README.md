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

### Enable Basic Authentication

```bash
helm install cegw cegw/cegw \
  --set config.auth.enabled=true \
  --set config.auth.type=basic \
  --set config.auth.basicUsername=admin \
  --set config.auth.basicPassword=secret
```

### Enable OAuth2 Authentication

```bash
helm install cegw cegw/cegw \
  --set config.auth.enabled=true \
  --set config.auth.type=oauth2 \
  --set config.auth.oauth2Issuer=https://accounts.google.com \
  --set config.auth.oauth2Audience=your-app-id
```

## Configuration

See [values.yaml](charts/cegw/values.yaml) for all configuration options.

### Key parameters

| Parameter | Description | Default |
|-----------|-------------|------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `ghcr.io/michaelahli/cegw` |
| `image.tag` | Image tag | `1.0.0` |
| `service.grpcPort` | gRPC port | `50051` |
| `service.httpPort` | HTTP port | `8080` |
| `config.httpsProxy` | HTTPS proxy URL (supports http/https/socks5) | `""` |
| `config.httpProxy` | HTTP proxy URL (supports http/https/socks5) | `""` |
| `config.noProxy` | Comma-separated list of hosts to bypass proxy | `""` |
| `config.auth.enabled` | Enable authentication | `false` |
| `config.auth.type` | Auth type: `basic` or `oauth2` | `"basic"` |
| `config.auth.basicUsername` | Basic auth username | `""` |
| `config.auth.basicPassword` | Basic auth password | `""` |
| `config.auth.oauth2Issuer` | OAuth2 issuer URL | `""` |
| `config.auth.oauth2Audience` | OAuth2 audience | `""` |

## Uninstall

```bash
helm uninstall cegw
```
