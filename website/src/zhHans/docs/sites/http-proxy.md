---
title: 通用 HTTP 反向代理
order: 4
---

# 通用 HTTP 反向代理（http）

## 简介

`http` 模式是一个通用的 HTTP 反向代理，可以将请求按**路径前缀**路由到不同的上游地址，适合同时代理多个不同上游接口的场景。

## 配置项（settings）

> 对应 Go 配置类型：`HttpGeneralProxySettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `destination` | `string` | 无 | 默认目标 URL，当没有 `mappings` 匹配时使用。`destination` 与 `mappings` 至少需要配置一个 |
| `mappings` | `[]HttpGeneralProxyMapping` | `[]` | 路径映射列表，每项含 `path`（路径前缀）和 `destination`（目标 URL）|
| `redirect_action` | `RedirectAction` | `rewrite_or_follow` | 重定向处理策略，见下方说明 |

### 路径匹配规则

`mappings` 列表中，匹配采用**最长前缀优先**原则。若多个 `path` 都是请求路径的前缀，则取最长的那个。

### 重定向处理策略（redirect_action）

| 值 | 说明 |
|----|------|
| `follow_all` | 跟随所有重定向（Pavonis 代替客户端处理重定向） |
| `rewrite_or_follow` | **默认值**。对于指向本站上游的相对 URL 重定向进行重写；对于外部 URL 直接跟随 |
| `rewrite_only` | 只重写相对 URL 重定向，外部 URL 原样返回给客户端 |
| `none` | 直接将上游的重定向响应透传给客户端，不做任何处理 |

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
