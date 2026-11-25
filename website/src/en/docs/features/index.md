---
title: Features
order: 2
---

# Features

The stages a request passes through in Pavonis:

```text
client
  │
  ▼
select site by host / path_prefix ── no match ──► 404
  │
  ▼
static_files path ────────────────── match ────► resource limits ──► local file / static content
  │ no match
  ▼
resource limits ──────────────────── exceeded ──► 429
  │
  ▼
client auth (if enabled) ─────────── failed ────► 401
  │
  ▼
header modification → outbound connection (IP pool / request proxy) ──► upstream
(uploads and downloads are subject to bandwidth limits)
```

## Site Modes

The core capability of Pavonis is expressed through **site modes**. A single instance can run any number of sites simultaneously, each choosing a mode that covers container images, code hosting, ML platforms, package management and more.

→ [Site Modes Documentation](../sites/)

## Static Files

A site can serve a local file or fixed content directly at given paths, for example a homepage or a health check endpoint. These paths require no authentication, even if the site has client authentication enabled.

→ [StaticFileConfig Documentation](../config/sites#staticfileconfig-static-files)

## Outbound IP Pool

When your server holds a large IPv6 address block (e.g. a `/48` subnet from HE Tunnel Broker), you can enable the IP pool to let Pavonis select outbound IPs from the pool at random or by rule, effectively bypassing per-IP rate limits imposed by upstream platforms. Supports `random` and `ip_hash` (pinned by client IP) strategies, with per-site override support.

→ [IP Pool Documentation](./ip-pool)

## Resource Limits

Limit bandwidth, request rate and concurrent requests, per client or for all clients combined. Sites can also be put into quota groups to give different sites different quotas.

→ [Resource Limits Documentation](./rate-limiting)

## Authentication, Headers and Request Proxy

Container registry and `any_url` sites support client authentication, with hot-reloadable user files (no restart required to update the user list). Automatically strips reverse-proxy-related headers to prevent leaking proxy chain information to upstreams. Upstream requests can go through one or more request proxies, with automatic switching when one is unreachable.

→ [Auth, Headers and Request Proxy Documentation](./auth-and-headers)
