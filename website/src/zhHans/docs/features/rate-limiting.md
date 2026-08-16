---
title: 流量与请求限速
order: 2
---

# 流量与请求限速

Pavonis 支持两类限速：**出站流量限速**（控制带宽）和**请求频率限速**（控制 QPS）。两者均在 [`ResourceLimitConfig`](../config/resource-limit) 中全局配置，任一条件触发即生效。

## 出站流量限速

使用**令牌桶**算法对出站流量进行速率控制。令牌桶的工作方式：桶以 `traffic_avg_mibps` 的速率持续填充令牌，每发送 1 MiB 数据消耗 1 个令牌；桶满时多余的令牌被丢弃，最大容量为 `traffic_burst_mib`。

| 字段 | 说明 |
|------|------|
| `traffic_avg_mibps` | 长期平均速率（令牌填充速率，单位 MiB/s） |
| `traffic_burst_mib` | 允许的突发容量（桶容量，单位 MiB） |
| `traffic_max_mibps` | 峰值速率上限（单位 MiB/s），不设则不限峰值 |

```yaml
resource_limit:
  traffic_avg_mibps: 100   # 平均 100 MiB/s
  traffic_burst_mib: 200   # 允许突发至 200 MiB
  traffic_max_mibps: 300   # 峰值不超过 300 MiB/s
```

三个字段均为可选，不设置即表示该维度不限制。

## 请求频率限制

对全局入站请求数量进行限制，支持按**秒、分钟、小时**三个粒度同时生效：

| 字段 | 说明 |
|------|------|
| `request_per_second` | 每秒最多请求数 |
| `request_per_minute` | 每分钟最多请求数 |
| `request_per_hour` | 每小时最多请求数 |

```yaml
resource_limit:
  request_per_second: 50
  request_per_minute: 1000
  request_per_hour: 10000
```

多个条件同时生效，任一条件触发时请求将被拒绝。不设置相关字段即表示不限制该项。

## 参考

- [`ResourceLimitConfig` 完整文档](../config/resource-limit)
