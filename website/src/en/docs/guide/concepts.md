---
title: Core Concepts
order: 3
---

# Core Concepts

This page explains the fundamental concepts in a Pavonis config file. After reading it, you should be able to understand and write a basic working configuration.

## Upstream and Downstream

These docs follow the usual reverse proxy convention (as in Nginx, Caddy and Envoy):

- **Downstream**: the client sending requests to Pavonis, such as `docker`, `pip` or a browser
- **Upstream**: the target server Pavonis forwards requests to, such as Docker Hub, GitHub or PyPI

```
client (downstream) ──▶ Pavonis ──▶ target server (upstream)
```

Config keys like `upstream_v2_url` and terms like "upstream request" follow this convention. A proxy that sits between clients and Pavonis (Nginx, a CDN) is called a **front proxy**.

## Site

A **site** is the basic routing unit in Pavonis. Each site binds to one or more hostnames (`host`), selects a working mode (`mode`), and carries any mode-specific settings (`settings`). Pavonis routes incoming requests to the matching site based on the `Host` header and optional path prefix.

```yaml
sites:
  - id: dockerhub            # unique site identifier for logs (optional)
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      upstream_v2_url: https://registry-1.docker.io/v2
```

A site can bind **multiple hostnames**, or set `host` to `"*"` as a catch-all for requests whose `Host` does not match any other site:

```yaml
sites:
  - host:
      - proxy.example.com
      - proxy2.example.com
    mode: github

  - host: '*'
    mode: http
    settings:
      destination: https://upstream.example.com
```

## Mode

The `mode` field determines the proxy behaviour of a site. See [Site Modes](../sites/) for the available modes and their parameters.

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

See [`SiteConfig`](../config/sites) for the format and matching rules.

## self_url

`self_url` is the fully accessible URL of this site as seen from the outside, in the format `scheme://host` (no path, no trailing `/`).

Some modes use it to rewrite callback URLs in responses (such as the auth realm in container registries, or redirect links in Hugging Face), so that the client's subsequent requests still go through Pavonis.

See [`SiteConfig`](../config/sites#when-self-url-is-required) for when `self_url` is required.

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
