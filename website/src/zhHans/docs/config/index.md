---
title: 配置概览
order: 4
---

# 配置概览

Pavonis 使用 YAML 格式的配置文件，通过 `-c` 参数指定路径（默认 `config.yml`）。

也可以通过环境变量 `PAVONIS_CONFIG` 直接提供配置文件的内容。设置了该变量时，`-c` 指定的文件会被忽略。

## 顶层字段

| 字段 | 类型 | 是否必填 | 说明 |
|------|------|----------|------|
| `debug` | `bool` | 否 | 是否开启 Debug 日志，默认 `false` |
| `server` | [`ServerConfig`](./server) | 否 | 服务器监听、入站连接与可信代理配置 |
| `request` | [`RequestConfig`](./request) | 否 | 出站请求配置（代理、IP 池、请求头修改） |
| `response` | [`ResponseConfig`](./response) | 否 | 响应配置（最大重定向次数、响应头修改） |
| `resource_limit` | [`ResourceLimitConfig`](./resource-limit) | 否 | 带宽、请求频率、并发请求数与请求超时限制 |
| `diagnostics` | [`DiagnosticsConfig`](./diagnostics) | 否 | 诊断服务配置 |
| `sites` | [`[]SiteConfig`](./sites) | **是** | 代理站点列表 |

> `Config` 为顶层配置结构体。

## 站点文件

站点配置较长时，可以把它移到独立的 YAML 文件中，在 `sites` 列表里用 `file` 引用。文件内容是一个完整的站点，写法与 `sites` 中的一项相同。引用项只能包含 `file`，站点的所有字段都写在文件里，效果等同于把文件内容写在引用所在的位置。

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

`file` 的相对路径以 `-c` 指定的配置文件所在目录为基准，通过 `PAVONIS_CONFIG` 提供配置时则以工作目录为基准。站点文件中的其他路径（如 `static_files` 的 `file`、`users_file`）不会改为以站点文件所在目录为基准。

## 完整配置示例

以下是一份涵盖多种功能的完整配置文件示例（所有域名已脱敏）：

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
  # GitHub 文件加速
  - id: ghproxy
    host: gh.example.com
    mode: github
    settings:
      size_limit: 104857600

  # Docker Hub 代理
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com

  # GHCR 代理
  - id: ghcr
    host: ghcr.example.com
    mode: container_registry_single
    self_url: https://ghcr.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token

  # 任意仓库代理
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com

  # Minecraft 认证聚合代理
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

  # PyPI 镜像
  - id: pypi
    host: pypi.example.com
    mode: pypi

  # Hugging Face 下载代理
  - id: huggingface
    host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com

  # 测速（与 PyPI 共用主机，通过路径前缀区分）
  - host: pypi.example.com
    mode: speed_test
    path_prefix: /speedtest
```
