---
title: ServerConfig (服务器设置)
order: 1
---

# ServerConfig (服务器设置)

`server` 部分配置 Pavonis 的监听地址和可信上游代理。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `listen` | `string` | `:8009` | 监听地址，格式为 `host:port`。`0.0.0.0:8009` 监听所有网卡，`:8009` 等同于 `0.0.0.0:8009` |
| `trusted_proxy_ips` | `[]string` | `["127.0.0.1/24"]` | 可信上游代理 IP 列表（支持 CIDR），来自这些地址的请求中的代理头部会被信任。填 `["*"]` 表示信任所有来源 |
| `trusted_proxy_headers` | `[]string` | 见下方 | 从哪些请求头中读取真实客户端 IP |

**`trusted_proxy_headers` 默认值：**

```yaml
- 'CF-Connecting-IP'   # Cloudflare 真实 IP
- 'X-Forwarded-For'    # 标准代理头
- 'X-Real-IP'          # 常见替代头
```

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
    - '*'              # 信任所有来源（Cloudflare 出口 IP 较多，可用 * 简化）
  trusted_proxy_headers:
    - 'CF-Connecting-IP'
```

### 部署在内网 Nginx 后面

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
    - '192.168.1.0/24'
  trusted_proxy_headers:
    - 'X-Real-IP'
```

## 注意事项

- `trusted_proxy_ips` 的作用是让 Pavonis 信任来自指定 IP 的代理头部，从而正确获取客户端真实 IP（主要用于日志和 `ip_hash` 策略）。
- 如果 Pavonis 直接暴露在公网，不建议将 `trusted_proxy_ips` 设为 `["*"]`，否则任何客户端都可以伪造 `X-Forwarded-For` 等头部。
