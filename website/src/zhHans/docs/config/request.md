---
title: RequestConfig (出站请求设置)
order: 2
---

# RequestConfig (出站请求设置)

`request` 部分配置 Pavonis 向上游发出请求时的行为，包括请求代理、IP 池和请求头修改。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `proxy` | `RequestProxyConfig` | 见下方 | 上游请求的代理配置。未配置任何 `servers` 时，上游请求直接连接。[详见下方](#requestproxyconfig) |
| `ip_pool` | `IpPoolConfig` | 见下方 | IP 池配置，[详见下方](#ippoolconfig-ip-池) |
| `header` | `HeaderModificationConfig` | 见下方 | 出站请求头修改配置，[详见下方](#headermodificationconfig-请求头修改) |

---

## RequestProxyConfig

让上游请求经过代理服务器。配置多个代理时，请求按配置顺序使用第一个可用的代理。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `servers` | `[]RequestProxyServerConfig` | `[]` | 代理服务器列表，按优先级从高到低排列 |
| `direct_fallback` | `bool` | `false` | 所有代理都不可用时，是否改为直连 |
| `connect_timeout` | `duration` | `5s` | 连接代理服务器的超时时间 |
| `block` | `RequestProxyBlockConfig` | 见下方 | 暂时跳过不可用代理的规则 |

**RequestProxyServerConfig**

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `url` | `string` | 无 | 代理地址，例如 `http://127.0.0.1:7890`。scheme 必须是 `http`、`https`、`socks5`、`socks5h` 之一。认证信息可写成 `http://user:pass@host:port` |
| `proxy_protocol` | `ProxyProtocol` | `none` | 连接此代理时发送的 PROXY protocol 版本，用于传递客户端 IP。可选 `none`（关闭）、`v1`、`v2`；代理的端口须启用对应版本。直连时不会发送 |

**RequestProxyBlockConfig**

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `failure_threshold` | `int` | `3` | 连续连接失败多少次后屏蔽该代理 |
| `cooldown` | `duration` | `10s` | 代理第一次被屏蔽的时长 |
| `max_cooldown` | `duration` | `5m` | 屏蔽时长的上限；同一个代理每次再被屏蔽，时长都会增加 |

**切换规则**

请求使用 `servers` 中第一个未被屏蔽的代理。代理连续连接失败 `failure_threshold` 次后被屏蔽并跳过；屏蔽结束后会重新尝试，再次失败则屏蔽时长翻倍，上限为 `max_cooldown`。所有代理都被屏蔽时，只有启用了 `direct_fallback` 才会直连，否则请求仍经由代理发送。

连接代理失败时，不带请求体的请求会改用下一个代理重新发送，启用 `direct_fallback` 时最后改为直连；带请求体的请求直接返回错误。

只有连不上代理才计为失败，包括超过 `connect_timeout`。连接建立后代理返回的错误，例如 `407`、`502` 或拒绝 `CONNECT`，不影响代理的可用性。

---

## IpPoolConfig (IP 池)

允许 Pavonis 从指定的子网中选取出站 IP 地址。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | `bool` | `false` | 是否启用 IP 池 |
| `default_strategy` | `IpPoolStrategy` | `none` | 默认出站 IP 选取策略，可被站点级 `ip_pool_strategy` 覆盖 |
| `subnets` | `[]string` | `[]` | 出站 IP 的 CIDR 子网列表 |

**策略（strategy）枚举值：**

| 值 | 说明 |
|----|------|
| `none` | 不使用 IP 池，使用系统默认出站 IP |
| `random` | 每次请求从池内随机选取一个 IP |
| `ip_hash` | 根据客户端 IP 哈希，同一客户端始终使用相同的出站 IP |

---

## HeaderModificationConfig (请求头修改)

控制 Pavonis 向上游发送请求时的头部内容。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `modify` | `map[string]string` | `{}` | 添加或覆盖指定请求头 |
| `delete` | `[]string` | 见下方 | 删除指定请求头 |

**`header.delete` 默认删除的头部（防止代理信息泄露给上游）：**

```yaml
# 通用反代头
- 'Via'
- 'X-Forwarded-For'
- 'X-Forwarded-Proto'
- 'X-Forwarded-Host'
# Cloudflare 相关头
- 'CDN-Loop'
- 'CF-Connecting-IP'
- 'CF-Connecting-IPv6'
- 'CF-EW-Via'
- 'CF-IPCountry'
- 'CF-Pseudo-IPv4'
- 'Cf-Ray'
- 'CF-Visitor'
- 'Cf-Warp-Tag-Id'
```

::: warning
显式配置 `header.delete` 会**完全替换**默认列表，而不是追加。如需保留默认删除行为并额外删除其他头部，需手动把默认列表也写进去。
:::

## 配置示例

### 使用请求代理

```yaml
request:
  proxy:
    servers:
      - url: http://127.0.0.1:7890
```

向代理传递客户端 IP 时，可配置 `proxy_protocol`。代理的接收端口须支持并启用对应版本的 PROXY protocol。

```yaml
request:
  proxy:
    servers:
      - url: http://127.0.0.1:7890
        proxy_protocol: v2
```

客户端 IP 的识别沿用 `server.trusted_proxy_ips` 和 `server.trusted_proxy_headers`，详见[服务器配置](./server)。

### 使用多个请求代理，并在最后回退到直连

```yaml
request:
  proxy:
    servers:
      - url: http://10.0.0.1:7890
        proxy_protocol: v2
      - url: socks5://10.0.0.2:1080
    direct_fallback: true
```

优先级依次为 `10.0.0.1`、`10.0.0.2`、直连。

### 配置 IP 池（随机策略）

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: random
    subnets:
      - '2001:db8::/48'
```

### 配置 IP 池（不同站点不同策略）

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: none     # 默认不使用 IP 池
    subnets:
      - '2001:db8::/48'

sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    ip_pool_strategy: ip_hash  # 此站点使用 ip_hash
```

### 向上游添加自定义请求头

```yaml
request:
  header:
    modify:
      X-Custom-Header: 'my-value'
      Authorization: 'Bearer secret-token'
```

### 自定义删除头部列表

```yaml
request:
  header:
    delete:
      - 'Via'
      - 'X-Forwarded-For'
      - 'X-Forwarded-Proto'
      - 'X-Forwarded-Host'
      - 'CDN-Loop'
      - 'CF-Connecting-IP'
      - 'CF-Connecting-IPv6'
      - 'CF-EW-Via'
      - 'CF-IPCountry'
      - 'CF-Pseudo-IPv4'
      - 'Cf-Ray'
      - 'CF-Visitor'
      - 'Cf-Warp-Tag-Id'
      - 'X-My-Internal-Header'  # 额外删除自定义头部
```
