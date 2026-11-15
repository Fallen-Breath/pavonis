---
title: General HTTP Reverse Proxy
order: 4
---

# General HTTP Reverse Proxy (http)

## Overview

The `http` mode is a general-purpose HTTP reverse proxy that routes requests to different upstream addresses based on **path prefixes**, suitable for aggregating multiple upstream interfaces behind a single site.

## Settings

> Corresponding Go config type: `HttpSiteSettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `destination` | `string` | none | Default target URL used when no `mappings` entry matches. At least one of `destination` or `mappings` must be configured |
| `mappings` | `[]HttpSiteMapping` | `[]` | Path mappings, each a `path` prefix and its `destination` URL |
| `redirect_action` | `RedirectAction` | `rewrite_or_follow` | Redirect handling strategy, see [below](#redirect-handling-strategy-redirect-action) |
| `body_rewrites` | `[]HttpSiteBodyRewrite` | `[]` | Response body text replacement rules, see [below](#response-body-rewriting-body-rewrites) |

### Path Matching Rules

A mapping `path` follows the same rules as the site's [`path_prefix`](../config/sites).

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

### Response Body Rewriting (body_rewrites)

Replaces fixed literal text in upstream response bodies, e.g. to swap upstream addresses or names hardcoded in pages for your own.

Each rule has the following fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `paths` | `[]string` | `[]` | Path prefixes the rule applies to. Same rules as a mapping `path`, matched against the request path, not the upstream path. Empty means all paths |
| `content_types` | `[]string` | `text/*`, `application/json`, `application/javascript`, `application/xml` | Response `Content-Type`s the rule applies to, written as `type/subtype`, `type/*` or `*/*` |
| `replacements` | `[]HttpSiteBodyReplacement` | `[]` | Replacements, each a `search` text and its `replace` text |

When a response matches multiple rules, all their replacements apply together. If several `search` texts match at the same position, the one configured first wins. Replaced text is not matched again.

::: warning
- Requests matching `paths` lose Range support, which breaks resumed and segmented downloads, so keep `paths` as specific as possible
- Replacement works on raw bytes, without charset conversion
:::

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

### Rewrite Upstream Addresses in Pages

The upstream docs pages hardcode absolute CDN URLs. Load those assets through this site's `/static` as well:

```yaml
sites:
  - host: www.example.com
    mode: http
    settings:
      destination: https://www.upstream.com
      mappings:
        - path: '/static'
          destination: https://cdn.upstream.com
      body_rewrites:
        - paths: ['/docs']   # rewrite docs pages only, downloads under /static are unaffected
          content_types: ['text/html']
          replacements:
            - search: 'https://cdn.upstream.com/'
              replace: '/static/'
            - search: 'Upstream Inc.'
              replace: 'Example'
```
