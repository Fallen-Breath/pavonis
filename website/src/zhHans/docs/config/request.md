---
title: RequestConfig (出站请求设置)
order: 2
---

# RequestConfig (出站请求设置)

`request` 部分配置 Pavonis 向上游发出请求时的行为，包括出站代理、IP 池和请求头修改。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `proxy` | `string` | 无 | 出站 HTTP 代理地址，例如 `http://127.0.0.1:7890`。所有上游请求将经过此代理 |
| `ip_pool` | `IpPoolConfig` | 见下方 | IP 池配置，[详见下方](#ipoolconfig-ip-池) |
| `header` | `HeaderModificationConfig` | 见下方 | 出站请求头修改配置，[详见下方](#headermodificationconfig-请求头修改) |

---

## IpPoolConfig (IP 池)

允许 Pavonis 从指定的 IPv6 子网中选取出站 IP 地址。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | `bool` | `false` | 是否启用 IP 池。启用时 `subnets` 不能为空 |
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

### 使用出站代理

```yaml
request:
  proxy: http://127.0.0.1:7890
```

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
