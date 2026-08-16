---
title: Container Registry Proxy (Any)
order: 3
---

# Container Registry Proxy — Any (container_registry_any)

## Overview

The `container_registry_any` mode enables transparent proxying of **any** OCI-compatible container registry by **embedding the target registry's hostname in the request path**. It is ideal for building a universal image accelerator — clients simply change the image address to use it.

**This mode supports pull only, not push. At the HTTP level this means only `GET` and `HEAD` requests are accepted.**

## Request Format

Request paths follow the format:

```
/{registry-host}/v2/{image-name}/{...}
```

For example, if the `container_registry_any` site is bound to `mirror.example.com`:

```bash
# Pull a Docker Hub official image
docker pull mirror.example.com/registry-1.docker.io/library/nginx:latest

# Pull a GHCR image
docker pull mirror.example.com/ghcr.io/user/image:tag

# Pull an NVCR image
docker pull mirror.example.com/nvcr.io/nvidia/pytorch:latest
```

## Settings

> Corresponding Go config type: `ContainerRegistryAnyProxySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `allow_list` | `bool` | `false` | Whether to allow the `_catalog` and `tags/list` endpoints |
| `repos_whitelist` | `[]string` | `[]` | Repository allowlist (includes registry hostname) |
| `repos_blacklist` | `[]string` | `[]` | Repository denylist (includes registry hostname) |
| `auth` | [`ContainerRegistryAuthConfig`](../config/auth) | none | Client access authentication config |

### Allow/Deny List Format

Entries are in the form `registry-host/namespace/repo`, with `*` wildcards supported:

```yaml
repos_whitelist:
  - 'ghcr.io/my-org/*'
  - 'registry-1.docker.io/library/*'
```

## Configuration Examples

### Basic Usage

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
```

### Allow Only Specific Registries

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
    settings:
      repos_whitelist:
        - 'ghcr.io/*'
        - 'registry-1.docker.io/library/*'
```

### Enable Client Authentication

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
    settings:
      auth:
        enabled: true
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 5m
```

For full authentication configuration, see [ContainerRegistryAuthConfig](../config/auth).
