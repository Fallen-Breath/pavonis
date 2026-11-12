---
title: Container Registry Proxy (Single)
order: 2
---

# Container Registry Proxy — Single (container_registry_single)

## Overview

The `container_registry_single` mode turns Pavonis into a proxy for a **fixed upstream** OCI/Docker container registry. It is ideal for setting up a dedicated mirror for a specific registry such as Docker Hub, GHCR or NVCR.

This mode fully supports the OCI Distribution Spec (v1/v2 API) and is compatible with `docker pull/push`, `crictl`, `containerd` and other container tools.

**When none of `upstream_v2_url`, `upstream_v1_url` and `upstream_auth_realm_url` is set, the site proxies Docker Hub.** These three URLs describe one registry, so to proxy another registry, set `upstream_v2_url` (plus the other two if needed); nothing falls back to Docker Hub then.

## Settings

> Corresponding Go config type: `ContainerRegistrySingleProxySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `upstream_v2_url` | `string` | see above | The upstream registry's `/v2` endpoint URL (no trailing `/`). Required when either of the other two URLs is set |
| `upstream_v1_url` | `string` | see above | The upstream registry's `/v1` endpoint URL (optional, no trailing `/`) |
| `upstream_auth_realm_url` | `string` | auto-discovered | Upstream auth realm URL; if omitted, Pavonis discovers it from the upstream `401` response |
| `allow_push` | `bool` | `false` | Whether to allow push operations (`PUT`/`POST`/`PATCH`/`DELETE` requests under `/v2`). Does not affect `/auth` |
| `allow_list` | `bool` | `false` | Whether to allow listing: the `_catalog` and `tags/list` endpoints, and the `/v1` API (used by `docker search`) |
| `repos_whitelist` | `[]string` | `[]` | Repository allowlist |
| `repos_blacklist` | `[]string` | `[]` | Repository denylist |
| `auth` | [`ClientAuthConfig`](../config/auth) | none | Client access authentication config |

### Allow/Deny List Format

Entries are in the form `namespace/repo`, with `*` wildcards supported. A bare `namespace` matches all repos under that namespace:

```yaml
repos_whitelist:
  - 'library/nginx'       # this repo and all repos below it
  - 'my-org/*'            # all repos under my-org
```

Both allowlists and denylists match by whole path-segment prefixes. For the entry `library/nginx`:

| Repository | Matches? |
|------------|----------|
| `library/nginx` | Yes |
| `library/nginx/extra` | Yes |
| `library/nginx/extra/deeper` | Yes |
| `library/nginx-extra` | No |
| `library/other` | No |

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

For full authentication configuration, see [ClientAuthConfig](../config/auth).

### Login-Free Private Mirror via Path Prefix

If you only need a registry mirror for Docker and don't want to run `docker login` on every machine, use a random string as the `path_prefix` so that it acts as an access key:

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    path_prefix: /Mys3cR3tPa55w0rd
    mode: container_registry_single
    self_url: https://docker.example.com
```

Then set the prefixed address as a mirror in `/etc/docker/daemon.json`:

```json
{
  "registry-mirrors": ["https://docker.example.com/Mys3cR3tPa55w0rd"]
}
```

From then on, Docker Hub pulls such as `docker pull nginx` go through this site automatically. Requests without the prefix don't match this site (they get a `404` if no other site is bound to the same `host`).

This **only works with Docker's `registry-mirrors`**: a registry address with a path is not a common convention, and other tools such as `containerd`, `podman` and `crictl` most likely can't use it. Also note that `registry-mirrors` itself only applies to Docker Hub images.

The prefix is effectively a password shared by everyone: anyone who can read `daemon.json` or the access logs can use it, and the only way to revoke it is to change the prefix. If you need per-user access or per-user revocation, use [client authentication](#enable-client-authentication) instead.

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
