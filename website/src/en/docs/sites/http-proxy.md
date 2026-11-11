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

A mapping `path` follows the same rules as `path_prefix`: it must start with `/`, must not end with `/`, and matches whole path segments, e.g. `/auth` matches `/auth/foo` but not `/authx`.

Matching is **longest-prefix-first**. If multiple `path` values match the request path, the longest one wins.

### Redirect Handling Strategy (redirect_action)

| Value | Description |
|-------|-------------|
| `follow_all` | Follow all redirects (Pavonis handles redirects on behalf of the client) |
| `rewrite_or_follow` | **Default.** Rewritable redirects are rewritten. All others are followed by Pavonis |
| `rewrite_only` | Rewritable redirects are rewritten. All others are returned to the client as-is |
| `none` | Pass upstream redirect responses through to the client unchanged |

A **rewritable redirect** is one whose `Location` points under the mapping's `destination`. After rewriting, `Location` becomes a path on this site, so the client's next request still goes through Pavonis. For a mapping `/auth` → `https://authserver.mojang.com`:

```text
Upstream Location: https://authserver.mojang.com/foo
Returned to client: /auth/foo
```

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
