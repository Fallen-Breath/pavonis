---
title: SiteConfig
order: 5
---

# SiteConfig

`sites` is a list where each entry configures one proxy site or references a separate [site file](./#site-files). Pavonis routes incoming requests to the matching site based on the `Host` header and path prefix.

## Fields

Each site shares the following common fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | No | Unique site identifier used as a log prefix. Auto-generated as `site0`, `site1`, etc. if omitted |
| `mode` | [`SiteMode`](../sites/) | **Yes** | Site working mode |
| `host` | `string \| []string` | **Yes** | Bound hostname(s). Can be a single string or a list. Use `"*"` to match all hosts (wildcard) |
| `self_url` | `string` | Depends | Externally accessible URL of this site, in `scheme://host` format (no path, no trailing `/`) |
| `path_prefix` | `string` | No | Path prefix for running multiple sites on one host. Must start with `/` and not end with `/`. Matches whole path segments: `/pypi` matches `/pypi` and `/pypi/...`, but not `/pypi2`. Special characters such as non-ASCII must be URL-escaped, e.g. `/%E4%B8%AD`. Empty matches all paths |
| `resource_group` | `string` | No | The [resource group](./resource-limit#resourcegroupconfig-resource-group) of this site. If omitted, only `global` applies |
| `ip_pool_strategy` | [`IpPoolStrategy`](./request#ippoolconfig-ip-pool) | No | IP pool outbound strategy for this site, overrides global `request.ip_pool.default_strategy`. Requires the global IP pool to be enabled |
| `static_files` | [`[]StaticFileConfig`](#staticfileconfig-static-files) | No | Local files or fixed content served directly at given paths of this site |
| `settings` | `object` | Depends | Mode-specific settings, documented on each mode's page |

## When self_url is Required

`self_url` **must** be configured in the following cases:

| Condition | Reason |
|-----------|--------|
| `mode: container_registry_single` | Auth realm URL must be rewritten to point to this site |
| `mode: container_registry_any` | Auth realm URL must be rewritten to point to this site |
| `mode: hugging_face` | Redirect and Link header URLs must be rewritten |
| `mode: github` with `settings.raw_text_url_rewrite: true` | URLs in text content must be rewritten to point to this site |

## Routing Priority

- When multiple sites bind the same `host`, the site with the **longest `path_prefix`** wins
- A wildcard host (`host: '*'`) only handles requests whose `Host` matches no other site. Once the `Host` matches, a request that fits none of that host's `path_prefix` values gets `404` rather than falling back to the wildcard site

## StaticFileConfig (Static Files)

A site can serve a local file or content written in the config directly at given paths, for example a homepage, a `robots.txt` or a health check endpoint. All other requests are handled by the site mode as usual.

`path` is relative to `path_prefix` and matches the request path exactly, ignoring the query. Static files require no authentication, even if the site has client authentication enabled.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | `string` | **Yes** | Request path. Must start with `/`, and special characters are written the same way as in `path_prefix`. Must be unique within the site |
| `file` | `string` | Either this or `content` | Local file path; relative paths are resolved against the working directory. Changes to the file take effect without a restart |
| `content` | `string` | Either this or `file` | Content returned as is |
| `content_type` | `string` | No | `Content-Type` of the response. If omitted, it is inferred from the extension for `file`, and is `text/plain; charset=utf-8` for `content` |

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
    mode: github
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

### Static Homepage and Health Check

```yaml
sites:
  - host: mirrors.example.com
    mode: http
    static_files:
      - path: /
        file: /srv/pavonis/index.html
      - path: /healthz
        content: ok
    settings:
      mappings:
        - path: /ubuntu
          destination: https://archive.ubuntu.com/ubuntu
```
