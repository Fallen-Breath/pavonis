---
title: 出站 IP 池
order: 1
---

# 出站 IP 池

Pavonis 默认使用系统出站 IP 向上游发起请求。当你持有大段 IPv6 地址（例如 HE Tunnel Broker 分配的 `/48` 子网）时，可以启用 **IP 池**，让 Pavonis 从池内按策略选取出站 IP，从而规避上游平台对单一 IP 的速率限制或访问封锁。

`random` 策略每次请求随机选取出站 IP，适合将请求分散到尽量多的 IP；`ip_hash` 策略让同一客户端始终使用同一出站 IP，适合需要上游识别"同一用户"的场景（例如某些需要会话一致性的 API）。全局默认策略可以按站点单独覆盖。

```text
random：每次请求随机
  客户端 A ─┬─ 请求 1 ──► 2001:db8::1a
            └─ 请求 2 ──► 2001:db8::7f
  客户端 B ─── 请求 1 ──► 2001:db8::05

ip_hash：同一客户端固定
  客户端 A ─┬─ 请求 1 ──► 2001:db8::1a
            └─ 请求 2 ──► 2001:db8::1a
  客户端 B ─── 请求 1 ──► 2001:db8::33
```

配置写法见 [`IpPoolConfig`](../config/request#ippoolconfig-ip-池) 与站点的 [`ip_pool_strategy`](../config/sites)。
