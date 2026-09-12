---
title: SiteConfig
order: 5
---

# SiteConfig

`sites` is a list where each entry configures one proxy site. Pavonis routes incoming requests to the matching site based on the `Host` header and path prefix.

## Fields

Each site shares the following common fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | No | Unique site identifier used as a log prefix. Auto-generated as `site0`, `site1`, etc. if omitted |
| `mode` | [`SiteMode`](#mode-enum) | **Yes** | Site working mode; see enum values below |
| `host` | `string \| []string` | **Yes** | Bound hostname(s). Can be a single string or a list. Use `"*"` to match all hosts (wildcard) |
| `self_url` | `string` | Depends | Externally accessible URL of this site, in `scheme://host` format (no path, no trailing `/`) |
| `path_prefix` | `string` | No | Path prefix filter; must start with `/`. Used to distinguish multiple sites on the same host |
| `ip_pool_strategy` | [`IpPoolStrategy`](#ip_pool_strategy-enum) | No | IP pool outbound strategy for this site, overrides global `request.ip_pool.default_strategy`. Requires the global IP pool to be enabled |
| `settings` | `object` | Depends | Mode-specific configuration; see each mode's documentation |

## mode Enum

| Value | Description |
|-------|-------------|
| `gh_proxy` | GitHub asset acceleration |
| `container_registry_single` | Single-target container registry proxy |
| `container_registry_any` | Any-target container registry proxy |
| `http` | General HTTP reverse proxy |
| `any_proxy` | Any upstream URL proxy |
| `pypi` | PyPI index mirror |
| `hugging_face` | HuggingFace download proxy |
| `speed_test` | Upload/download speed test |

## When self_url is Required

`self_url` **must** be configured in the following cases:

| Condition | Reason |
|-----------|--------|
| `mode: container_registry_single` | Auth realm URL must be rewritten to point to this site |
| `mode: container_registry_any` | Auth realm URL must be rewritten to point to this site |
| `mode: hugging_face` | Redirect and Link header URLs must be rewritten |
| `mode: gh_proxy` with `settings.raw_text_url_rewrite: true` | URLs in text content must be rewritten to point to this site |

## ip_pool_strategy Enum

| Value | Description |
|-------|-------------|
| `none` | Do not use the IP pool |
| `random` | Randomly select a pool IP |
| `ip_hash` | Pin outbound IP by client IP hash |

## Routing Priority

- When multiple sites bind the same `host`, the site with the **longest `path_prefix`** wins
- A wildcard host (`host: '*'`) has lower priority than exact hostname matches

## Configuration Examples

### Basic Site

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

### Bind Multiple Hostnames

```yaml
sites:
  - id: ghproxy
    host:
      - gh.example.com
      - gh2.example.com
    mode: gh_proxy
```

### Same Host, Multiple Path Prefixes

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi

  - host: proxy.example.com
    mode: speed_test
    path_prefix: /speedtest

  - host: proxy.example.com
    mode: http
    path_prefix: /cf
    settings:
      destination: https://speed.cloudflare.com
```

### Per-Site IP Pool Strategy

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    ip_pool_strategy: ip_hash

  - host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
    ip_pool_strategy: random
```
