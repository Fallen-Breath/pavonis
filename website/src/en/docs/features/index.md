---
title: Features
order: 2
---

# Features

## Site Modes

The core capability of Pavonis is expressed through **site modes**. A single instance can run any number of sites simultaneously, each choosing a mode that covers container images, code hosting, ML platforms, package management and more:

| Mode | Purpose |
|------|---------|
| `gh_proxy` | GitHub Releases, Raw files, Gist acceleration |
| `container_registry_single` | Proxy a specific OCI container registry (e.g. Docker Hub, GHCR) |
| `container_registry_any` | Universal container image proxy, routing by hostname embedded in the request path |
| `http` | General HTTP reverse proxy with path-based routing |
| `pypi` | PyPI index mirror, rewrites download links automatically |
| `hugging_face` | HuggingFace model and dataset download proxy |
| `speed_test` | Upload/download speed test endpoints |

→ [Site Modes Documentation](../sites/)

## Outbound IP Pool

When your server holds a large IPv6 address block (e.g. a `/48` subnet from HE Tunnel Broker), you can enable the IP pool to let Pavonis select outbound IPs from the pool at random or by rule, effectively bypassing per-IP rate limits imposed by upstream platforms. Supports `random` and `ip_hash` (pinned by client IP) strategies, with per-site override support.

→ [IP Pool Documentation](./ip-pool)

## Traffic and Request Rate Limiting

Control outbound bandwidth using a token bucket algorithm. Simultaneously supports per-second, per-minute and per-hour request count limits to prevent the proxy from consuming excessive resources or being abused. Both rate limits are optional — leave them unset to impose no limit.

→ [Rate Limiting Documentation](./rate-limiting)

## Authentication, Headers and Outbound Proxy

Container registry sites support client authentication with built-in hot-reload of user files (no restart required to update the user list). Automatically strips reverse-proxy-related headers to prevent leaking proxy chain information to upstreams. A single outbound proxy can be configured for all upstream requests.

→ [Auth, Headers and Outbound Proxy Documentation](./auth-and-headers)
