---
title: 流量与请求限速
order: 2
---

# 流量与请求限速

Pavonis 支持两类限速：流量限速（控制带宽）和请求频率限速（控制 QPS），均在 [`ResourceLimitConfig`](../config/resource-limit) 中配置。

限速分为两个层级：

- 每客户端：流量限速与请求频率限速按客户端分别计算，每个客户端拥有独立的额度
- 全局：流量可额外设置所有客户端合计的总上限

一个请求需要同时满足所有已配置的限制。

## 客户端的划分

客户端按来源 IP 划分：IPv4 按单个地址，IPv6 按所在的 `/64` 网段。同一 `/64` 网段内的 IPv6 地址共享同一份额度。

位于反向代理之后时，客户端 IP 取决于 [`server`](../config/server) 中受信任代理的配置。

## 流量限速

使用令牌桶算法控制流量速率：桶以 `traffic_avg_mibps` 的速率持续填充令牌，每传输 1 MiB 数据消耗 1 个令牌；桶的容量为 `traffic_burst_mib`，空闲时积累的令牌允许短时间内超过平均速率。

流量按上传与下载合计计算，两个方向共用同一份额度。

### 每客户端流量限速

| 字段 | 说明 |
|------|------|
| `traffic_avg_mibps` | 长期平均速率（令牌填充速率，单位 MiB/s） |
| `traffic_burst_mib` | 突发容量（桶容量，单位 MiB） |
| `traffic_max_mibps` | 峰值速率上限（单位 MiB/s） |

```yaml
resource_limit:
  traffic_avg_mibps: 10    # 每个客户端平均 10 MiB/s
  traffic_burst_mib: 20    # 每个客户端允许突发 20 MiB
  traffic_max_mibps: 30    # 每个客户端峰值不超过 30 MiB/s
```

### 全局流量限速

限制所有客户端的流量总和，适用于需要控制服务器总带宽的场景。字段含义与每客户端流量限速一一对应：

| 字段 | 说明 |
|------|------|
| `global_traffic_avg_mibps` | 所有客户端合计的长期平均速率（MiB/s） |
| `global_traffic_burst_mib` | 所有客户端合计的突发容量（MiB） |
| `global_traffic_max_mibps` | 所有客户端合计的峰值速率上限（MiB/s） |

```yaml
resource_limit:
  traffic_avg_mibps: 10           # 每个客户端平均 10 MiB/s
  global_traffic_avg_mibps: 100   # 所有客户端合计平均 100 MiB/s
  global_traffic_burst_mib: 50
```

## 请求频率限速

按客户端限制请求数量，支持秒、分钟、小时三个粒度同时生效：

| 字段 | 说明 |
|------|------|
| `request_per_second` | 每个客户端每秒最多请求数 |
| `request_per_minute` | 每个客户端每分钟最多请求数 |
| `request_per_hour` | 每个客户端每小时最多请求数 |

```yaml
resource_limit:
  request_per_second: 50
  request_per_minute: 1000
  request_per_hour: 10000
```

任一条件超限时，请求将被拒绝并返回 `429 Too Many Requests`。

## 适用范围

所有字段均为可选，不设置即表示不限制该项。

以上限速作用于所有代理类站点。[测速](../sites/speed-test)站点不受限速影响。

## 参考

- [`ResourceLimitConfig` 完整文档](../config/resource-limit)
