---
title: ResourceLimitConfig (资源限制)
order: 4
---

# ResourceLimitConfig (资源限制)

`resource_limit` 部分配置带宽、请求频率、并发请求数与请求超时。`global` 中的配额对所有站点生效，资源组中的配额只对加入该组的站点生效，两者同时约束一个请求。各项限制的工作方式详见[资源限制](../features/rate-limiting)。

所有字段均为可选。配额中除请求超时默认 `1h` 外，其余限制不配置即不限。

## ResourceLimitConfig

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `global` | [`ResourceQuotaConfig`](#resourcequotaconfig-配额) | 请求超时 `1h` | 对所有站点生效的配额 |
| `groups` | [`[]ResourceGroupConfig`](#resourcegroupconfig-资源组) | 无 | 资源组列表，站点通过 [`resource_group`](./sites) 加入其中一个 |
| `max_concurrent_429_responses` | `int` | `64` | 同时发送的 `429` 拒绝响应数上限。超出后，新拒绝的请求会被直接中止而不返回响应 |
| `idle_client_cache_size` | `int` | `10240` | 最多保留多少份暂无请求的 `per_client` 用量。超出后，最久未访问的用量会被清零 |

## ResourceQuotaConfig (配额)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `request_timeout` | `duration` | `global` 中为 `1h`，资源组中不额外限制 | 单个请求的最长处理时间，使用 Go duration 格式，如 `30m`、`2h`、`90s`。资源组中配置时，取它与全局超时中较短的一个 |
| `per_client` | [`ClientQuotaConfig`](#clientquotaconfig-限制项) | 不限 | 每个客户端各自一份配额的限制 |
| `all_clients` | [`ClientQuotaConfig`](#clientquotaconfig-限制项) | 不限 | 所有客户端共用一份配额的限制 |

## ResourceGroupConfig (资源组)

资源组包含 `ResourceQuotaConfig` 的全部字段，另有：

| 字段 | 类型 | 是否必填 | 说明 |
|------|------|----------|------|
| `id` | `string` | **是** | 组名，供站点的 `resource_group` 引用 |

## ClientQuotaConfig (限制项)

### 带宽（令牌桶算法）

流量按上传与下载合计计算。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `traffic_avg_mibps` | `float` | 不限 | 平均速率（MiB/s），即令牌桶的填充速率 |
| `traffic_burst_mib` | `float` | 见下文 | 突发容量（MiB），即令牌桶的桶容量 |
| `traffic_max_mibps` | `float` | 不限 | 峰值速率上限（MiB/s） |

`traffic_burst_mib` 未设置时取 `traffic_avg_mibps` 与 `0.125` 中的较小值，仅在设置了 `traffic_avg_mibps` 时生效。

### 请求

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `request_per_second` | `float` | 不限 | 每秒最大请求数 |
| `request_per_minute` | `float` | 不限 | 每分钟最大请求数 |
| `request_per_hour` | `float` | 不限 | 每小时最大请求数 |
| `request_concurrency` | `int` | 不限 | 同时处理的最大请求数 |

请求超出其中任意一项即返回 `429 Too Many Requests`。

## 配置示例

### 限制每个客户端的带宽

```yaml
resource_limit:
  global:
    per_client:
      traffic_avg_mibps: 10      # 每个客户端平均 10 MiB/s
      traffic_burst_mib: 20      # 每个客户端允许突发最多 20 MiB
      traffic_max_mibps: 30      # 即使有突发，每个客户端峰值也不超过 30 MiB/s
```

### 限制服务器总带宽

```yaml
resource_limit:
  global:
    per_client:
      traffic_avg_mibps: 10      # 每个客户端平均 10 MiB/s
    all_clients:
      traffic_avg_mibps: 100     # 所有客户端合计平均 100 MiB/s
      traffic_burst_mib: 50
```

### 限制请求频率与并发

```yaml
resource_limit:
  global:
    per_client:
      request_per_second: 20
      request_per_minute: 500
      request_concurrency: 8
```

### 设置请求超时

```yaml
resource_limit:
  global:
    request_timeout: 30m    # 超过 30 分钟的请求将被终止
```

### 为部分站点单独设置配额

```yaml
resource_limit:
  global:
    per_client:
      request_per_second: 20
  groups:
    - id: downloads
      all_clients:
        request_concurrency: 50
        traffic_avg_mibps: 200

sites:
  - host: gh.example.com
    mode: github
    resource_group: downloads
  - host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
    resource_group: downloads
  # 其他 host 不受 downloads 资源组的影响
```

GitHub 与 Hugging Face 的请求合计最多同时处理 50 个，总带宽不超过 200 MiB/s，同时仍受 `global` 中每个客户端每秒 20 个请求的限制。
