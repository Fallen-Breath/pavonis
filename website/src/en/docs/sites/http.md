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

Trailing slashes are stripped from `destination` and `mappings[].destination`.

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

Replaces fixed literal text in upstream response bodies, e.g. to swap upstream addresses or names hardcoded in pages or API responses for your own.

Each rule has the following fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `paths` | `[]string` | `[]` | Path prefixes the rule applies to. Same rules as a mapping `path`, matched against the request path, not the upstream path. Empty means all paths |
| `content_types` | `[]string` | see description | Response `Content-Type`s the rule applies to, written as `type/subtype`, `type/*` or `*/*`. Defaults to `text/*`, `application/json`, `application/javascript`, `application/xml`, or `application/json` when `json_paths` is set |
| `json_paths` | `[]string` | none | Restricts replacement to JSON string values at these paths instead of the whole body, see [below](#json-path-rewriting-json-paths) |
| `replacements` | `[]HttpSiteBodyReplacement` | `[]` | Replacements, each a `search` text and its `replace` text |

When a response matches multiple rules, rules with `json_paths` run before rules without `json_paths`. If several `json_paths` rules match the same JSON field, the one configured first wins. Rules without `json_paths` apply together, with the earlier `search` winning at the same position and replaced text not matched again.

::: warning
- Requests matching `paths` lose Range support, which breaks resumed and segmented downloads, so keep `paths` as specific as possible
- Replacement works on raw bytes, without charset conversion
:::

#### JSON Path Rewriting (json_paths)

When `json_paths` is set, replacement applies only to JSON string values at the matched paths, leaving non-string and unmatched fields untouched. JSON string escapes are decoded before replacement and re-encoded into valid JSON strings afterward.

A path starts at the JSON root and chains child selectors, with an optional leading `$` or `$.`:

| Selector | Matches | Examples |
|----------|---------|----------|
| `name`, `.name` | A specific object key | `url`, `$.meta.url` |
| `["name"]`, `['name']` | An object key with special characters like `.` or `*` | `meta["*"]`, `$['a.b']` |
| `*`, `[*]` | Any object key or any array element | `files[*].url`, `$.*.*.url` |

Examples of what paths match and do not match:

| Path | Matches | Does not match |
|------|---------|----------------|
| `url` | `{"url": "a"}` | `{"url": 123}`, `{"data": {"url": "a"}}` |
| `files[*].url` | `{"files": [{"url": "a"}]}`, `{"files": {"v1": {"url": "a"}}}` | `{"files": {"url": "a"}}` |
| `meta["*"]` | `{"meta": {"*": "a"}}` | `{"meta": {"url": "a"}}` |
| `$['a.b']` | `{"a.b": "a"}` | `{"a": {"b": "a"}}` |

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

### Rewrite Upstream Addresses in JSON Responses

The upstream API returns JSON containing upstream URLs. Rewrite only the download URL fields without touching other content:

```yaml
sites:
  - host: api.example.com
    mode: http
    settings:
      destination: https://api.upstream.com
      body_rewrites:
        - paths: ['/manifest']
          json_paths: ['files[*].url']
          replacements:
            - search: 'https://api.upstream.com/'
              replace: 'https://api.example.com/'
```

Upstream response:

```json
{
  "homepage": "https://api.upstream.com/",
  "files": [
    {
      "name": "app.tar.gz",
      "url": "https://api.upstream.com/app.tar.gz"
    }
  ]
}
```

Returned to client:

```json
{
  "homepage": "https://api.upstream.com/",
  "files": [
    {
      "name": "app.tar.gz",
      "url": "https://api.example.com/app.tar.gz"
    }
  ]
}
```
