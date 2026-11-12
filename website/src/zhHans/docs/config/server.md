---
title: ServerConfig (服务器设置)
order: 1
---

# ServerConfig (服务器设置)

`server` 部分配置 Pavonis 的监听地址，以及位于客户端与 Pavonis 之间的可信前置代理（如 Nginx、CDN）。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `listen` | `string` | `:8009` | 监听地址，格式为 `host:port`。`:8009` 与 `0.0.0.0:8009` 均监听所有网卡 |
| `trusted_proxy_ips` | `[]string` | `["127.0.0.1/24"]` | 可信前置代理 IP 列表（支持 CIDR），来自这些地址的请求中的代理头部会被信任。填 `["*"]` 表示信任所有来源 |
| `trusted_proxy_headers` | `[]string` | `[]` | 从哪些请求头中读取真实客户端 IP，按顺序取第一个存在且含有合法 IP 的头 |

## 获取客户端真实 IP

部署在前置代理后面时，Pavonis 看到的连接来源是前置代理，需要配置 `trusted_proxy_headers`，才能从请求头中得到客户端真实 IP。客户端 IP 用于日志、`ip_hash` 策略和单客户端限速。

`trusted_proxy_headers` 中只应列出前置代理每次都会设置的头。前置代理不处理的头会原样到达 Pavonis，客户端可以随意伪造。例如前置是 Nginx 时，客户端可以自带一个 `CF-Connecting-IP`。

`X-Forwarded-For` 中可能有多个 IP。Pavonis 从右往左读取，跳过 `trusted_proxy_ips` 中的地址，取第一个不可信的 IP。经过多层前置代理时，需要把每一层的地址都加进 `trusted_proxy_ips`。

如果 Pavonis 直接暴露在公网，不要将 `trusted_proxy_ips` 设为 `["*"]`，否则任何客户端都可以伪造 `trusted_proxy_headers` 中的头。

## 配置示例

### 最简配置

```yaml
server:
  listen: 0.0.0.0:8009
```

### 部署在 Cloudflare 后面

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '*'
  trusted_proxy_headers:
    - 'CF-Connecting-IP'
```

`*` 只适用于源站只能经 Cloudflare 访问的情况，例如通过 Cloudflare Tunnel 接入，或防火墙只放行 Cloudflare 的 IP。否则客户端可以绕过 Cloudflare 直连源站，伪造 `CF-Connecting-IP`。这时请把 `trusted_proxy_ips` 填成 [Cloudflare 公布的 IP 段](https://www.cloudflare.com/ips/)。

### 部署在 Nginx 后面

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Real-IP'
```

Nginx 需要设置该头，并保留 `Host`：

```nginx
proxy_set_header Host $host;
proxy_set_header X-Real-IP $remote_addr;
```

### 部署在 Caddy、Traefik 等会追加 X-Forwarded-For 的代理后面

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Forwarded-For'
```
