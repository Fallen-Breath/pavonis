---
title: Container Registry Proxy (Single)
order: 2
---

# Container Registry Proxy — Single (container_registry_single)

## Overview

The `container_registry_single` mode turns Pavonis into a proxy for a **fixed upstream** OCI/Docker container registry. It is ideal for setting up a dedicated mirror for a specific registry such as Docker Hub, GHCR or NVCR.

This mode fully supports the OCI Distribution Spec (v1/v2 API) and is compatible with `docker pull/push`, `crictl`, `containerd` and other container tools.

**Omitting all `settings` proxies Docker Hub by default.**

## Settings

> Corresponding Go config type: `ContainerRegistrySingleProxySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `upstream_v2_url` | `string` | Docker Hub | The upstream registry's `/v2` endpoint URL (no trailing `/`) |
| `upstream_v1_url` | `string` | Docker Hub | The upstream registry's `/v1` endpoint URL (optional, no trailing `/`) |
| `upstream_auth_realm_url` | `string` | auto-discovered | Upstream auth realm URL; if omitted, Pavonis discovers it from the upstream `401` response |
| `allow_push` | `bool` | `false` | Whether to allow push operations (`PUT`/`POST`/`PATCH`/`DELETE` requests) |
| `allow_list` | `bool` | `false` | Whether to allow the `_catalog` and `tags/list` endpoints |
| `repos_whitelist` | `[]string` | `[]` | Repository allowlist |
| `repos_blacklist` | `[]string` | `[]` | Repository denylist |
| `auth` | [`ContainerRegistryAuthConfig`](../config/auth) | none | Client access authentication config |

### Allow/Deny List Format

Entries are in the form `namespace/repo`, with `*` wildcards supported. A bare `namespace` matches all repos under that namespace:

```yaml
repos_whitelist:
  - 'library/nginx'       # exact match
  - 'my-org/*'            # all repos under my-org
```

## Configuration Examples

### Proxy Docker Hub (default)

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

Usage:

```bash
docker pull docker.example.com/library/nginx:latest
```

Or configure as a registry mirror in `/etc/docker/daemon.json`:

```json
{
  "registry-mirrors": ["https://docker.example.com"]
}
```

### Proxy GHCR

```yaml
sites:
  - id: ghcr
    host: ghcr.example.com
    mode: container_registry_single
    self_url: https://ghcr.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token
```

### Proxy NVCR (NVIDIA)

```yaml
sites:
  - id: nvcr
    host: nvcr.example.com
    mode: container_registry_single
    self_url: https://nvcr.example.com
    settings:
      upstream_v2_url: https://nvcr.io/v2
      upstream_auth_realm_url: https://nvcr.io/proxy_auth
```

### Proxy a Self-Hosted Registry (e.g. Codeberg)

```yaml
sites:
  - id: codeberg
    host: codeberg.example.com
    mode: container_registry_single
    self_url: https://codeberg.example.com
    settings:
      upstream_v2_url: https://codeberg.org/v2
```

### Enable Client Authentication

When enabled, clients must supply a username and password to use this proxy:

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      auth:
        enabled: true
        users:
          - name: alice
            password: my-secret-password
```

For full authentication configuration, see [ContainerRegistryAuthConfig](../config/auth).

### Allow Only Specific Images

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      repos_whitelist:
        - 'library/*'     # official images only
        - 'my-org/*'
```
