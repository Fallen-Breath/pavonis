---
title: SiteConfig (站点配置)
order: 5
---

# SiteConfig (站点配置)

`sites` 是一个列表，每项配置一个代理站点。Pavonis 启动后根据请求的 `Host` 头部和路径前缀，将请求路由到对应的站点处理器。

## SiteConfig 字段

每个站点都包含以下公共字段：

| 字段 | 类型 | 是否必填 | 说明 |
|------|------|----------|------|
| `id` | `string` | 否 | 站点唯一标识符，用于日志前缀。若不填，自动生成 `site0`、`site1` 等 |
| `mode` | [`SiteMode`](#mode-枚举值) | **是** | 站点工作模式，可选值见下方 |
| `host` | `string \| []string` | **是** | 绑定的主机名，可以是单个字符串或字符串列表。填 `"*"` 表示匹配所有主机（通配符） |
| `self_url` | `string` | 视模式而定 | 本站对外可访问的 URL，格式为 `scheme://host`（不含路径，不含尾部 `/`） |
| `path_prefix` | `string` | 否 | 路径前缀过滤，必须以 `/` 开头。用于同一主机下区分多个站点 |
| `ip_pool_strategy` | [`IpPoolStrategy`](#ip_pool_strategy-枚举值) | 否 | 此站点的 IP 池出站策略，覆盖全局 `request.ip_pool.default_strategy`。需先启用全局 IP 池 |
| `settings` | `object` | 视模式而定 | 各模式特有的配置，详见各功能页面 |

## mode 枚举值

| 值 | 说明 |
|----|------|
| `gh_proxy` | GitHub 文件加速 |
| `container_registry_single` | 单目标容器镜像仓库代理 |
| `container_registry_any` | 任意容器镜像仓库代理 |
| `http` | 通用 HTTP 反向代理 |
| `pypi` | PyPI 包索引镜像 |
| `hugging_face` | HuggingFace 下载代理 |
| `speed_test` | 上传/下载测速 |

## self_url 必填说明

以下情况 **必须** 配置 `self_url`：

| 条件 | 原因 |
|------|------|
| `mode: container_registry_single` | 需要将认证 realm URL 重写为指向本站 |
| `mode: container_registry_any` | 需要将认证 realm URL 重写为指向本站 |
| `mode: hugging_face` | 需要重写重定向和 Link 头部中的 URL |
| `mode: gh_proxy` 且 `settings.raw_text_url_rewrite: true` | 需要将文本内容中的 URL 重写为指向本站 |

## ip_pool_strategy 枚举值

| 值 | 说明 |
|----|------|
| `none` | 不使用 IP 池 |
| `random` | 随机选取池内 IP |
| `ip_hash` | 根据客户端 IP 哈希固定出站 IP |

## 路由优先级

- 当多个站点绑定了相同的 `host` 时，**`path_prefix` 越长，优先级越高**
- 通配符主机（`host: '*'`）优先级低于精确匹配的主机名

## 配置示例

### 基础站点

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

### 绑定多个主机名

```yaml
sites:
  - id: ghproxy
    host:
      - gh.example.com
      - gh2.example.com
    mode: gh_proxy
```

### 同一主机，多个路径前缀

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

### 站点级 IP 池策略

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
