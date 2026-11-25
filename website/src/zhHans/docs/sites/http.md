---
title: 通用 HTTP 反向代理
order: 4
---

# 通用 HTTP 反向代理（http）

## 简介

`http` 模式是一个通用的 HTTP 反向代理，可以将请求按**路径前缀**路由到不同的上游地址，适合同时代理多个不同上游接口的场景。

## 配置项（settings）

> 对应 Go 配置类型：`HttpSiteSettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `destination` | `string` | 无 | 默认目标 URL，当没有 `mappings` 匹配时使用。`destination` 与 `mappings` 至少需要配置一个 |
| `mappings` | `[]HttpSiteMapping` | `[]` | 路径映射列表，每项含 `path`（路径前缀）和 `destination`（目标 URL）|
| `redirect_action` | `RedirectAction` | `rewrite_or_follow` | 重定向处理策略，见[下方说明](#重定向处理策略-redirect-action) |
| `body_rewrites` | `[]HttpSiteBodyRewrite` | `[]` | 响应体文本替换规则，见[下方说明](#响应体改写-body-rewrites) |

### 路径匹配规则

`mappings` 中的 `path` 与站点的 [`path_prefix`](../config/sites) 规则相同。

匹配采用**最长前缀优先**原则。若多个 `path` 都能匹配请求路径，则取最长的那个。

`destination` 和 `mappings[].destination` 末尾的 `/` 会被移除。

### 重定向处理策略（redirect_action）

| 值 | 说明 |
|----|------|
| `follow_all` | 跟随所有重定向（Pavonis 代替客户端处理重定向） |
| `rewrite_or_follow` | **默认值**。可重写的重定向被重写，其余由 Pavonis 跟随 |
| `rewrite_only` | 可重写的重定向被重写，其余原样返回给客户端 |
| `none` | 直接将上游的重定向响应透传给客户端，不做任何处理 |

**可重写的重定向**指 `Location` 指向该映射的 `destination` 之下。重写后 `Location` 变为本站路径，客户端的后续请求仍经过 Pavonis。例如映射 `/auth` → `https://authserver.mojang.com`：

```text
上游 Location: https://authserver.mojang.com/foo
返回客户端:    /auth/foo
```

### 响应体改写（body_rewrites）

对上游响应体做固定的字面量替换，适合把页面或接口中写死的上游地址、名称等换成本站的。

每条规则的字段如下：

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `paths` | `[]string` | `[]` | 生效的路径前缀，规则与 `mappings` 中的 `path` 相同，匹配的是请求路径而非上游路径。留空表示对所有路径生效 |
| `content_types` | `[]string` | 见说明 | 生效的响应 `Content-Type`，写法为 `type/subtype`、`type/*` 或 `*/*`。未设置 `json_paths` 时默认为 `text/*`、`application/json`、`application/javascript`、`application/xml`，设置 `json_paths` 时默认为 `application/json` |
| `json_paths` | `[]string` | 无 | 仅改写命中这些 JSON 路径的字符串值，而不是整份响应体，见[下方说明](#json-路径改写-json-paths) |
| `replacements` | `[]HttpSiteBodyReplacement` | `[]` | 替换列表，每项含 `search`（查找的文本）与 `replace`（替换为的文本） |

当一个响应命中多条规则时，带 `json_paths` 的规则先于不带 `json_paths` 的规则执行。同一个 JSON 字段命中多条 `json_paths` 规则时，以配置在前的那条为准。不带 `json_paths` 的多条规则会合并生效，同一位置能匹配多个 `search` 时以配置在前的为准，且替换后的文本不会再被匹配。

::: warning
- 命中 `paths` 的请求不再支持 Range，断点续传、分段下载会失效，因此 `paths` 应尽量写得精确
- 替换按字节进行，不做字符集转换
:::

#### JSON 路径改写（json_paths）

配置 `json_paths` 后，改写只作用于命中路径上的 JSON 字符串值，非字符串字段和未命中的字段保持原样。替换前会自动解码 JSON 字符串转义，替换后再编码回合法的 JSON 字符串。

路径从 JSON 根节点开始，由逐级选择器组成，开头的 `$` 或 `$.` 可省略：

| 选择器 | 匹配目标 | 示例 |
|--------|----------|------|
| `name`、`.name` | 对象的指定键 | `url`、`$.meta.url` |
| `["name"]`、`['name']` | 含 `.`、`*` 等特殊字符的对象键 | `meta["*"]`、`$['a.b']` |
| `*`、`[*]` | 对象的任意键或数组的任意元素 | `files[*].url`、`$.*.*.url` |

以下是几种常见路径的匹配结果：

| 路径 | 会匹配 | 不会匹配 |
|------|--------|----------|
| `url` | `{"url": "a"}` | `{"url": 123}`、`{"data": {"url": "a"}}` |
| `files[*].url` | `{"files": [{"url": "a"}]}`、`{"files": {"v1": {"url": "a"}}}` | `{"files": {"url": "a"}}` |
| `meta["*"]` | `{"meta": {"*": "a"}}` | `{"meta": {"url": "a"}}` |
| `$['a.b']` | `{"a.b": "a"}` | `{"a": {"b": "a"}}` |

## 配置示例

### 单一目标

将所有请求转发到同一个上游：

```yaml
sites:
  - host: speed.example.com
    mode: http
    path_prefix: /cfspeed
    settings:
      destination: https://speed.cloudflare.com
```

### 多路径映射（Minecraft 认证聚合代理）

将不同路径前缀的请求路由到不同的 Mojang 服务：

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

访问 `https://mc.example.com/auth/...` 会被转发到 `https://authserver.mojang.com/...`。

### 混用 destination 与 mappings

`destination` 作为默认后备，在没有 `mappings` 匹配时生效：

```yaml
sites:
  - host: api.example.com
    mode: http
    settings:
      destination: https://api.upstream.com    # 默认目标
      mappings:
        - path: '/v2'
          destination: https://api-v2.upstream.com   # /v2 路径优先匹配
```

### 控制重定向行为

```yaml
sites:
  - host: proxy.example.com
    mode: http
    settings:
      destination: https://upstream.example.com
      redirect_action: follow_all    # 让 Pavonis 跟随所有重定向
```

### 改写页面中的上游地址

上游文档页里写死了 CDN 的绝对地址，让这些资源也经由本站的 `/static` 加载：

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
        - paths: ['/docs']   # 只改写文档页，/static 下的资源下载不受影响
          content_types: ['text/html']
          replacements:
            - search: 'https://cdn.upstream.com/'
              replace: '/static/'
            - search: 'Upstream Inc.'
              replace: 'Example'
```

### 改写 JSON 接口中的上游地址

上游接口返回的 JSON 里包含上游域名，只改写下载链接字段而不影响其它内容：

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

上游返回的响应：

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

改写后返回客户端：

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
