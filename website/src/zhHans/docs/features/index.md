---
title: 功能特性
order: 2
---

# 功能特性

## 站点模式（Site Modes）

Pavonis 的核心能力通过**站点模式**来体现。一个实例可以同时运行任意数量的站点，每个站点选择一种模式，覆盖容器镜像、代码托管、机器学习平台、包管理等主流场景：

| 模式 | 用途 |
|------|------|
| `gh_proxy` | GitHub Releases、Raw 文件、Gist 加速 |
| `container_registry_single` | 代理指定 OCI 容器镜像仓库（如 Docker Hub、GHCR） |
| `container_registry_any` | 通用容器镜像加速，按路径中的主机名路由 |
| `http` | 通用 HTTP 反向代理，支持多路径映射 |
| `pypi` | PyPI 包索引镜像，自动重写下载链接 |
| `hugging_face` | HuggingFace 模型/数据集下载代理 |
| `speed_test` | 上传/下载测速端点 |

→ [详细文档：站点模式](../sites/)

## 出站 IP 池

当你的服务器持有大段 IPv6 地址时（例如 HE Tunnel Broker 分配的 `/48` 子网），可以启用 IP 池，让 Pavonis 从池内随机或按规则选取出站 IP，有效规避上游平台对单一 IP 的速率限制。支持 `random`（随机）和 `ip_hash`（按客户端 IP 固定映射）等策略，也可以按站点单独覆盖。

→ [详细文档：出站 IP 池](./ip-pool)

## 流量与请求限速

通过令牌桶算法控制出站带宽，同时支持按秒、分钟、小时设置请求数上限，防止代理服务占用过多资源或被滥用。两类限速均为可选，不设置即不限制。

→ [详细文档：流量与请求限速](./rate-limiting)

## 认证、请求头与出站代理

容器镜像站点支持客户端身份验证，内置用户文件热重载（无需重启更新用户列表）；自动清理反代相关头部，避免将代理链信息泄漏给上游；并可为所有上游请求指定统一的出站代理。

→ [详细文档：认证、请求头与出站代理](./auth-and-headers)

一个 Pavonis 实例可以同时运行任意数量的代理站点，每个站点独立配置 `host`、`mode` 和 `settings`。

通过 `path_prefix` 还可以让多个站点共用同一个域名，用路径前缀区分：

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi

  - host: proxy.example.com
    mode: speed_test
    path_prefix: /speedtest
```

详见 [SiteConfig (站点配置)](../config/sites)。

## 站点模式（Site Modes）

Pavonis 通过 `mode` 字段支持多种代理场景，覆盖容器镜像、代码托管、机器学习平台、包管理等：

| 模式 | 用途 |
|------|------|
| `gh_proxy` | GitHub Releases、Raw 文件、Gist 加速 |
| `container_registry_single` | 代理指定 OCI 容器镜像仓库（如 Docker Hub、GHCR） |
| `container_registry_any` | 通用容器镜像加速，按路径中的主机名路由 |
| `http` | 通用 HTTP 反向代理，支持多路径映射 |
| `pypi` | PyPI 包索引镜像，自动重写下载链接 |
| `hugging_face` | HuggingFace 模型/数据集下载代理 |
| `speed_test` | 上传/下载测速端点 |

每种模式的详细说明见 [站点模式](../sites/)。

## IP 池出站

当服务器拥有大段 IPv6 地址时（如 HE Tunnel Broker `/48` 子网），可以配置 IP 池，让 Pavonis 用池内 IP 作为出站地址，避免被上游平台基于 IP 限速。

支持三种选取策略：

| 策略 | 说明 |
|------|------|
| `none` | 不使用 IP 池（默认） |
| `random` | 每次请求随机选取一个出站 IP |
| `ip_hash` | 根据客户端 IP 哈希，同一客户端固定使用同一出站 IP |

可在全局设置默认策略，也可以按站点单独覆盖：

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: none
    subnets:
      - '2001:db8::/48'

sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    ip_pool_strategy: ip_hash   # 此站点覆盖为 ip_hash
```

详见 [RequestConfig (出站请求设置)](../config/request)。

## 流量限速

使用令牌桶算法对出站流量进行速率控制，防止代理服务占用过多带宽：

| 字段 | 说明 |
|------|------|
| `traffic_avg_mibps` | 长期平均速率（令牌填充速率） |
| `traffic_burst_mib` | 允许的突发容量（桶容量） |
| `traffic_max_mibps` | 峰值速率上限 |

```yaml
resource_limit:
  traffic_avg_mibps: 100   # 平均 100 MiB/s
  traffic_burst_mib: 200   # 最多突发 200 MiB
  traffic_max_mibps: 300   # 峰值不超过 300 MiB/s
```

详见 [ResourceLimitConfig (资源限制)](../config/resource-limit)。

## 请求频率限制

对全局请求数量进行限制，支持按秒、分钟、小时分别设置阈值，多个条件同时生效：

```yaml
resource_limit:
  request_per_second: 50
  request_per_minute: 1000
  request_per_hour: 10000
```

详见 [ResourceLimitConfig (资源限制)](../config/resource-limit)。

## 容器仓库访问认证

`container_registry_single` 和 `container_registry_any` 模式支持启用客户端认证，要求访问者提供用户名和密码。用户列表支持：

- **内联配置**：直接写在 `config.yml` 中
- **外部文件**：单独的 `users.yml`，支持**无需重启的热重载**

详见 [ContainerRegistryAuthConfig (容器仓库认证)](../config/auth)。

## 请求头清理

默认自动删除请求中携带的反代相关头部（`Via`、`X-Forwarded-For` 等）和 Cloudflare 注入的头部（`CF-Connecting-IP`、`CF-Ray` 等），避免将代理链信息泄露给上游。

支持自定义添加或删除任意请求头与响应头：

```yaml
request:
  header:
    modify:
      X-Custom-Token: 'my-token'
    delete:
      - 'X-Internal-Header'

response:
  header:
    modify:
      Access-Control-Allow-Origin: '*'
```

详见 [RequestConfig (出站请求设置)](../config/request) 和 [ResponseConfig (响应设置)](../config/response)。

## 出站代理

可以为所有上游请求指定一个统一的 HTTP 出站代理：

```yaml
request:
  proxy: http://127.0.0.1:7890
```

详见 [RequestConfig (出站请求设置)](../config/request)。
