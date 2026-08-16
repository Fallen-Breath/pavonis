---
title: 出站 IP 池
order: 1
---

# 出站 IP 池

## 概念

Pavonis 默认使用系统出站 IP 向上游发起请求。当你持有大段 IPv6 地址（例如 HE Tunnel Broker 分配的 `/48` 子网）时，可以启用 **IP 池**，让 Pavonis 从池内按策略选取出站 IP，从而规避上游平台对单一 IP 的速率限制或访问封锁。

## 配置

IP 池在 [`RequestConfig`](../config/request) 的 `ip_pool` 字段下配置：

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: random
    subnets:
      - '2001:db8::/48'
      - '2001:db8:1::/64'
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | `bool` | 是否启用 IP 池 |
| `default_strategy` | [`IpPoolStrategy`](../config/request) | 全局默认出站策略 |
| `subnets` | `[]string` | 可用的 IPv6 子网列表（CIDR 格式） |

## 出站策略

| 策略值 | 说明 |
|--------|------|
| `none` | 不使用 IP 池，走系统默认出站 IP（默认值） |
| `random` | 每次请求从池内随机选取一个出站 IP |
| `ip_hash` | 根据客户端 IP 哈希，同一客户端始终使用同一出站 IP |

`random` 适合希望将请求分散到尽量多的 IP 的场景；`ip_hash` 适合需要上游识别"同一用户"的场景（例如某些需要会话一致性的 API）。

## 按站点覆盖

全局 `default_strategy` 可以被单个站点的 `ip_pool_strategy` 字段覆盖，实现精细控制：

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: none       # 大多数站点默认不使用 IP 池
    subnets:
      - '2001:db8::/48'

sites:
  - host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
    ip_pool_strategy: random     # 此站点启用随机出站 IP

  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    ip_pool_strategy: ip_hash    # 此站点按客户端 IP 固定映射
```

## 参考

- [`RequestConfig` 完整文档](../config/request)
- [`SiteConfig` 站点级配置](../config/sites)
