---
title: SiteConfig (站点配置)
order: 5
---

# SiteConfig (站点配置)

`sites` 是一个列表，每项配置一个代理站点，也可以引用独立的[站点文件](./#站点文件)。Pavonis 启动后根据请求的 `Host` 头部和路径前缀，将请求路由到对应的站点处理器。

## SiteConfig 字段

每个站点都包含以下公共字段：

| 字段 | 类型 | 是否必填 | 说明 |
|------|------|----------|------|
| `id` | `string` | 否 | 站点唯一标识符，用于日志前缀。若不填，自动生成 `site0`、`site1` 等 |
| `mode` | [`SiteMode`](../sites/) | **是** | 站点工作模式 |
| `host` | `string \| []string` | **是** | 绑定的主机名，可以是单个字符串或字符串列表。填 `"*"` 表示匹配所有主机（通配符） |
| `self_url` | `string` | 视模式而定 | 本站对外可访问的 URL，格式为 `scheme://host`（不含路径，不含尾部 `/`） |
| `path_prefix` | `string` | 否 | 路径前缀，用于在同一主机下运行多个站点。必须以 `/` 开头，不能以 `/` 结尾。按完整路径段匹配：`/pypi` 匹配 `/pypi` 和 `/pypi/...`，不匹配 `/pypi2`。含中文等特殊字符时需按 URL 编码填写，如 `/%E4%B8%AD`。不填则匹配所有路径 |
| `ip_pool_strategy` | [`IpPoolStrategy`](./request#ippoolconfig-ip-池) | 否 | 此站点的 IP 池出站策略，覆盖全局 `request.ip_pool.default_strategy`。需先启用全局 IP 池 |
| `static_files` | [`[]StaticFileConfig`](#staticfileconfig-静态文件) | 否 | 在本站的指定路径上直接返回本地文件或固定内容 |
| `settings` | `object` | 视模式而定 | 各模式特有的配置，详见各功能页面 |

## self_url 必填说明

以下情况 **必须** 配置 `self_url`：

| 条件 | 原因 |
|------|------|
| `mode: container_registry_single` | 需要将认证 realm URL 重写为指向本站 |
| `mode: container_registry_any` | 需要将认证 realm URL 重写为指向本站 |
| `mode: hugging_face` | 需要重写重定向和 Link 头部中的 URL |
| `mode: github` 且 `settings.raw_text_url_rewrite: true` | 需要将文本内容中的 URL 重写为指向本站 |

## 路由优先级

- 当多个站点绑定了相同的 `host` 时，**`path_prefix` 越长，优先级越高**
- 通配符主机（`host: '*'`）只处理 `Host` 未匹配任何其他站点的请求。`Host` 一旦匹配，若请求不符合该主机下任何站点的 `path_prefix`，则返回 `404`，不会回落到通配符站点

## StaticFileConfig (静态文件)

站点可以在指定路径上直接返回本地文件或配置中写定的内容，例如首页、`robots.txt` 或健康检查接口。其余请求照常交给站点的模式处理。

`path` 相对 `path_prefix`，与请求路径精确匹配，不考虑 query。即使站点启用了客户端认证，访问静态文件也无需认证。

| 字段 | 类型 | 是否必填 | 说明 |
|------|------|----------|------|
| `path` | `string` | **是** | 请求路径，必须以 `/` 开头，特殊字符的写法与 `path_prefix` 相同。同一站点内不能重复 |
| `file` | `string` | 与 `content` 二选一 | 本地文件路径。文件修改后无需重启即可生效 |
| `content` | `string` | 与 `file` 二选一 | 直接返回的内容 |
| `content_type` | `string` | 否 | 响应的 `Content-Type`。不填时，`file` 按扩展名推断，`content` 为 `text/plain; charset=utf-8` |

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
    mode: github
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

### 静态首页与健康检查

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
