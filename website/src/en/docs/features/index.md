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
client auth (if enabled) ─────────── failed ────► 401
  │
  ▼
request rate limit ───────────────── exceeded ──► 429
  │
  ▼
header modification → outbound connection (IP pool / request proxy) ──► upstream
(uploads and downloads are subject to traffic limits)
```

## Site Modes

The core capability of Pavonis is expressed through **site modes**. A single instance can run any number of sites simultaneously, each choosing a mode that covers container images, code hosting, ML platforms, package management and more.

→ [Site Modes Documentation](../sites/)

## Outbound IP Pool

When your server holds a large IPv6 address block (e.g. a `/48` subnet from HE Tunnel Broker), you can enable the IP pool to let Pavonis select outbound IPs from the pool at random or by rule, effectively bypassing per-IP rate limits imposed by upstream platforms. Supports `random` and `ip_hash` (pinned by client IP) strategies, with per-site override support.

→ [IP Pool Documentation](./ip-pool)

## Traffic and Request Rate Limiting

Limit bandwidth and per-second, per-minute and per-hour request counts per client, and optionally cap the combined bandwidth of all clients, to keep the proxy from consuming excessive resources or being abused.

→ [Rate Limiting Documentation](./rate-limiting)

## Authentication, Headers and Request Proxy

Container registry and `any_proxy` sites support client authentication, with hot-reloadable user files (no restart required to update the user list). Automatically strips reverse-proxy-related headers to prevent leaking proxy chain information to upstreams. Upstream requests can go through one or more request proxies, with automatic switching when one is unreachable.

→ [Auth, Headers and Request Proxy Documentation](./auth-and-headers)
