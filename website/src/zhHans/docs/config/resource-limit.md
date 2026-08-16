---
title: ResourceLimitConfig (资源限制)
order: 4
---

# ResourceLimitConfig (资源限制)

`resource_limit` 部分配置全局的流量速率和请求频率限制，防止代理服务占用过多带宽或被过度请求。

**所有字段均为可选，不配置即表示不限制该项。配置的值必须大于 0（或 `-1` 禁用，仅适用于测速相关字段）。**

## 字段说明

### 流量限速（令牌桶算法）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `traffic_avg_mibps` | `float` | 不限 | 出站流量平均速率（MiB/s），即令牌桶的填充速率 |
| `traffic_burst_mib` | `float` | 不限 | 出站流量突发容量（MiB），即令牌桶的桶容量 |
| `traffic_max_mibps` | `float` | 不限 | 出站流量峰值速率（MiB/s），限制瞬时最大速率 |

流量限制使用令牌桶算法：
- `traffic_avg_mibps` 控制长期平均速率（令牌填充速率）
- `traffic_burst_mib` 控制允许的瞬时突发量（桶容量），在流量空闲时积累，允许短时间内超过平均速率
- `traffic_max_mibps` 可进一步限制峰值速率上限

### 请求频率限制

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `request_per_second` | `float` | 不限 | 全局每秒最大请求数 |
| `request_per_minute` | `float` | 不限 | 全局每分钟最大请求数 |
| `request_per_hour` | `float` | 不限 | 全局每小时最大请求数 |

多个请求频率限制可以同时配置，请求必须同时满足所有条件才能被处理。

### 超时

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `request_timeout` | `duration` | `1h` | 单次请求的最大处理时间。使用 Go duration 格式，如 `30m`、`2h`、`90s` |

## 配置示例

### 限制出站带宽

```yaml
resource_limit:
  traffic_avg_mibps: 100     # 平均 100 MiB/s
  traffic_burst_mib: 200     # 允许突发最多 200 MiB
```

### 完整流量限速

```yaml
resource_limit:
  traffic_avg_mibps: 50
  traffic_burst_mib: 100
  traffic_max_mibps: 200     # 即使有突发，峰值也不超过 200 MiB/s
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
