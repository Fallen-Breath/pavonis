---
title: General HTTP Reverse Proxy
order: 4
---

# General HTTP Reverse Proxy (http)

## Overview

The `http` mode is a general-purpose HTTP reverse proxy that routes requests to different upstream addresses based on **path prefixes**, suitable for aggregating multiple upstream interfaces behind a single site.

## Settings

> Corresponding Go config type: `HttpGeneralProxySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `destination` | `string` | none | Default target URL used when no `mappings` entry matches. At least one of `destination` or `mappings` must be configured |
| `mappings` | `[]HttpGeneralProxyMapping` | `[]` | Path mapping list; each entry has a `path` (prefix) and a `destination` URL |
| `redirect_action` | `RedirectAction` | `rewrite_or_follow` | Redirect handling strategy; see below |

### Path Matching Rules

The `mappings` list uses **longest-prefix-first** matching. If multiple `path` values are prefixes of the request path, the longest one wins.

### Redirect Handling Strategy (redirect_action)

| Value | Description |
|-------|-------------|
| `follow_all` | Follow all redirects (Pavonis handles redirects on behalf of the client) |
| `rewrite_or_follow` | **Default.** Rewrite relative-URL redirects pointing to the same upstream; follow external URL redirects |
| `rewrite_only` | Rewrite relative-URL redirects only; return external URL redirects to the client as-is |
| `none` | Pass upstream redirect responses through to the client unchanged |

## Configuration Examples

### Single Destination

Forward all requests to the same upstream:

```yaml
sites:
  - host: speed.example.com
    mode: http
    path_prefix: /cfspeed
    settings:
      destination: https://speed.cloudflare.com
```

### Multiple Path Mappings (Minecraft Auth Aggregation)

Route different path prefixes to different Mojang services:

```yaml
sites:
  - id: mcauth
    host: mc.example.com
    mode: http
    settings:
      mappings:
        - path: '/auth'
          destination: https://authserver.mojang.com
        - path: '/account'
          destination: https://api.mojang.com
        - path: '/session'
          destination: https://sessionserver.mojang.com
        - path: '/services'
          destination: https://api.minecraftservices.com
```

Requests to `https://mc.example.com/auth/...` are forwarded to `https://authserver.mojang.com/...`.

### Mixing destination and mappings

`destination` acts as the fallback when no `mappings` entry matches:

```yaml
sites:
  - host: api.example.com
    mode: http
    settings:
      destination: https://api.upstream.com      # default target
      mappings:
        - path: '/v2'
          destination: https://api-v2.upstream.com   # /v2 matched first
```

### Control Redirect Behaviour

```yaml
sites:
  - host: proxy.example.com
    mode: http
    settings:
      destination: https://upstream.example.com
      redirect_action: follow_all    # Pavonis follows all redirects
```
