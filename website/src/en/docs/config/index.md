---
title: Config Overview
order: 4
---

# Config Overview

Pavonis uses a YAML config file specified via the `-c` flag (default `config.yml`).

Alternatively, the content of the config file can be provided directly through the `PAVONIS_CONFIG` environment variable. When it is set, the file specified by `-c` is ignored.

## Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `debug` | `bool` | No | Enable debug logging. Default `false` |
| `server` | [`ServerConfig`](./server) | No | Server listen address and trusted proxy config |
| `request` | [`RequestConfig`](./request) | No | Outbound request config (proxy, IP pool, header modification) |
| `response` | [`ResponseConfig`](./response) | No | Response config (max redirects, response header modification) |
| `resource_limit` | [`ResourceLimitConfig`](./resource-limit) | No | Bandwidth, request rate, concurrent request and request timeout limits |
| `diagnostics` | [`DiagnosticsConfig`](./diagnostics) | No | Diagnostics service config |
| `sites` | [`[]SiteConfig`](./sites) | **Yes** | Proxy sites |

> `Config` is the top-level config struct.

## Site Files

A long site config can be moved into a separate YAML file and referenced from the `sites` list with `file`. The file holds one complete site, written the same way as an entry in `sites`. A reference entry can only contain `file`, with all fields of the site written in the file. The result is the same as writing the file content in place of the reference.

```yaml
# /etc/pavonis/config.yml
sites:
  - file: sites/mirrors.yml
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

```yaml
# /etc/pavonis/sites/mirrors.yml
id: mirrors
host: mirrors.example.com
mode: http
settings:
  mappings:
    - path: /ubuntu
      destination: https://archive.ubuntu.com/ubuntu
```

A relative `file` path is resolved against the directory of the config file given by `-c`, or against the working directory when the config is provided through `PAVONIS_CONFIG`. Other paths in a site file, such as `file` in `static_files` and `users_file`, are not resolved against the site file's directory.

## Full Config Example

A complete config file covering multiple features (all domain names are anonymised):

```yaml
debug: false

server:
  listen: 0.0.0.0:8009
  trusted_proxy_ips:
    - '127.0.0.1/24'
    - '10.0.0.0/8'
  trusted_proxy_headers:
    - 'X-Forwarded-For'

request:
  ip_pool:
    enabled: true
    default_strategy: none
    subnets:
      - '2001:db8::/48'

resource_limit:
  global:
    request_timeout: 30m
    per_client:
      traffic_avg_mibps: 100
      traffic_burst_mib: 50
      traffic_max_mibps: 200
      request_per_second: 100

diagnostics:
  enabled: true
  listen: 127.0.0.1:6009

sites:
  # GitHub asset acceleration
  - id: ghproxy
    host: gh.example.com
    mode: github
    settings:
      size_limit: 104857600

  # Docker Hub proxy
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com

  # GHCR proxy
  - id: ghcr
    host: ghcr.example.com
    mode: container_registry_single
    self_url: https://ghcr.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token

  # Universal registry mirror
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com

  # Minecraft auth aggregation proxy
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

  # PyPI mirror
  - id: pypi
    host: pypi.example.com
    mode: pypi

  # Hugging Face download proxy
  - id: huggingface
    host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com

  # Speed test (shares host with PyPI, distinguished by path prefix)
  - host: pypi.example.com
    mode: speed_test
    path_prefix: /speedtest
```
