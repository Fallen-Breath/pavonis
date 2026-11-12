---
title: ResourceLimitConfig (资源限制)
order: 4
---

# ResourceLimitConfig (资源限制)

`resource_limit` 部分配置流量速率、请求频率限制以及请求超时，防止代理服务占用过多带宽或被过度请求。

流量与请求频率限制按客户端分别计算（IPv4 按单个地址，IPv6 按 `/64` 网段）；此外可为流量设置一个所有客户端合计的全局上限。限速的工作方式详见[流量与请求限速](../features/rate-limiting)。

所有限速字段均为可选，不配置即表示不限制该项。

## 字段说明

### 流量限速（令牌桶算法）

每个客户端的流量限速，流量按上传与下载合计计算。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `traffic_avg_mibps` | `float` | 不限 | 每个客户端的平均速率（MiB/s），即令牌桶的填充速率 |
| `traffic_burst_mib` | `float` | 见下文 | 每个客户端的突发容量（MiB），即令牌桶的桶容量 |
| `traffic_max_mibps` | `float` | 不限 | 每个客户端的峰值速率上限（MiB/s） |

`traffic_burst_mib` 未设置时取 `traffic_avg_mibps` 与 `0.125` 中的较小值，仅在设置了 `traffic_avg_mibps` 时生效。

### 全局流量限速

所有客户端合计的流量限速，各字段含义与上方的每客户端流量限速相同，默认值规则也相同。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `global_traffic_avg_mibps` | `float` | 不限 | 所有客户端合计的平均速率（MiB/s） |
| `global_traffic_burst_mib` | `float` | 见上文 | 所有客户端合计的突发容量（MiB） |
| `global_traffic_max_mibps` | `float` | 不限 | 所有客户端合计的峰值速率上限（MiB/s） |

### 请求频率限制

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `request_per_second` | `float` | 不限 | 每个客户端每秒最大请求数 |
| `request_per_minute` | `float` | 不限 | 每个客户端每分钟最大请求数 |
| `request_per_hour` | `float` | 不限 | 每个客户端每小时最大请求数 |

多个请求频率限制可以同时配置，请求必须同时满足所有条件才能被处理，否则返回 `429 Too Many Requests`。

### 超时

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `request_timeout` | `duration` | `1h` | 单次请求的最大处理时间。使用 Go duration 格式，如 `30m`、`2h`、`90s` |

## 配置示例

### 限制每个客户端的带宽

```yaml
resource_limit:
  traffic_avg_mibps: 10      # 每个客户端平均 10 MiB/s
  traffic_burst_mib: 20      # 每个客户端允许突发最多 20 MiB
```

### 完整流量限速

```yaml
resource_limit:
  traffic_avg_mibps: 10
  traffic_burst_mib: 20
  traffic_max_mibps: 30      # 即使有突发，每个客户端峰值也不超过 30 MiB/s
```

### 限制服务器总带宽

```yaml
resource_limit:
  traffic_avg_mibps: 10            # 每个客户端平均 10 MiB/s
  global_traffic_avg_mibps: 100    # 所有客户端合计平均 100 MiB/s
  global_traffic_burst_mib: 50
```

### 限制请求频率

```yaml
resource_limit:
  request_per_second: 20
  request_per_minute: 500
  request_per_hour: 5000
```

### 设置请求超时

```yaml
resource_limit:
  request_timeout: 30m    # 超过 30 分钟的请求将被终止
```

### 综合示例

```yaml
resource_limit:
  traffic_avg_mibps: 100
  traffic_burst_mib: 50
  request_per_second: 50
  request_timeout: 1h
```
