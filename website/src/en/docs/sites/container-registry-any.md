---
title: Container Registry Proxy (Any)
order: 3
---

# Container Registry Proxy — Any (container_registry_any)

## Overview

The `container_registry_any` mode proxies any OCI-compatible container registry, which makes it a good fit for a universal image accelerator. Clients write the source registry in the image name, so no per-registry site is needed.

**This mode is only for pulling public upstream images**: push is not supported (only `GET` and `HEAD` requests are accepted), and neither is logging in to an upstream account to pull private images.

## Usage

Write the image name as `{pavonis address}/{source registry}/{image name}`. For example, with the site bound to `mirror.example.com`:

```bash
# Docker Hub official image
docker pull mirror.example.com/docker.io/library/nginx:latest

# GHCR image
docker pull mirror.example.com/ghcr.io/user/image:tag

# NVCR image
docker pull mirror.example.com/nvcr.io/nvidia/pytorch:latest
```

The source registry must be a full hostname, i.e. it contains `.` or `:`, or is `localhost`. A Docker Hub image must therefore be written in full as `docker.io/library/nginx`, not `nginx` as with plain docker. `docker.io` and `registry-1.docker.io` are equivalent.

containerd transparent mirrors (mirror entries in `hosts.toml`) cannot use this mode, because the image name carries no source registry in that setup. Listing endpoints such as `_catalog` and `tags/list` are not supported either.

## Settings

> Corresponding Go config type: `ContainerRegistryAnyProxySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `repos_whitelist` | `[]string` | `[]` | Repository allowlist (includes registry hostname) |
| `repos_blacklist` | `[]string` | `[]` | Repository denylist (includes registry hostname) |
| `auth` | [`ClientAuthConfig`](../config/auth) | none | Client access authentication config |

### Allow/Deny List Format

Entries are in the form `registry/namespace/repo`, with `*` wildcards supported. `docker.io` and `registry-1.docker.io` are treated as the same registry:

```yaml
repos_whitelist:
  - 'ghcr.io/my-org/*'
  - 'docker.io/library/*'
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
        - 'docker.io/library/*'
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

With auth enabled, clients need to run `docker login mirror.example.com` first, using their Pavonis account.

For full authentication configuration, see [ClientAuthConfig](../config/auth).
