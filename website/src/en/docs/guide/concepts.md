---
title: Core Concepts
order: 2
---

# Core Concepts

This page explains the fundamental concepts in a Pavonis config file. After reading it, you should be able to understand and write a basic working configuration.

## Site

A **site** is the basic routing unit in Pavonis. Each site binds to one or more hostnames (`host`), selects a working mode (`mode`), and carries any mode-specific settings (`settings`). Pavonis routes incoming requests to the matching site based on the `Host` header and optional path prefix.

```yaml
sites:
  - id: dockerhub            # unique site identifier for logs (optional)
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      upstream_v2_url: https://registry-1.docker.io
```

A site can bind **multiple hostnames**, or set `host` to `"*"` as a catch-all handler that matches any request not claimed by another site:

```yaml
sites:
  - host:
      - proxy.example.com
      - proxy2.example.com
    mode: gh_proxy

  - host: '*'
    mode: http
    settings:
      mappings:
        - prefix: /
          destination: https://upstream.example.com
```

## Mode

The `mode` field determines the proxy behaviour of a site. Currently supported modes:

| Mode | Description |
|------|-------------|
| `gh_proxy` | GitHub Releases / Raw files / Gist acceleration |
| `container_registry_single` | Proxy a specific container registry (e.g. Docker Hub, GHCR) |
| `container_registry_any` | Universal container image proxy, routing by hostname in the request path |
| `http` | General HTTP reverse proxy with path-based routing |
| `pypi` | PyPI index mirror, rewrites download links automatically |
| `hugging_face` | HuggingFace model and dataset download proxy |
| `speed_test` | Upload/download speed test endpoints |

For detailed parameters of each mode, see [Site Modes](../sites/).

## Path Prefix (path_prefix)

`path_prefix` allows different paths under the same domain to be routed to different sites, enabling "one domain, multiple proxy services":

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi

  - host: proxy.example.com
    mode: speed_test
    path_prefix: /speedtest
```

- Must begin with `/`
- Longer prefixes take higher priority (longest prefix match)

## self_url

`self_url` is the fully accessible URL of this site as seen from the outside, in the format `scheme://host` (no path, no trailing `/`).

Some modes use it to rewrite callback URLs in responses (such as the auth realm in container registries, or redirect links in HuggingFace), so that the client's subsequent requests still go through Pavonis.

| Mode | Requires `self_url` |
|------|---------------------|
| `container_registry_single` | ✅ Required |
| `container_registry_any` | ✅ Required |
| `hugging_face` | ✅ Required |
| `gh_proxy` (when `raw_text_url_rewrite` is enabled) | ✅ Required |

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com   # scheme + host only, no trailing slash
```

## Next Steps

With the basics in hand, explore further:

- [Features](../features/) — IP pool, rate limiting, authentication and other cross-site capabilities
- [Site Modes](../sites/) — Detailed parameters and examples for each mode
- [Config Reference](../config/) — Complete configuration struct documentation
