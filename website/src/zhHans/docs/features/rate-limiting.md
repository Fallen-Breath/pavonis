---
title: 资源限制
order: 2
---

# 资源限制

资源限制可以控制带宽、请求频率、并发请求数和请求超时，防止代理服务占用过多资源或被滥用。各项限制均为可选，字段与配置示例见 [`ResourceLimitConfig`](../config/resource-limit)。[测速](../sites/speed-test)站点的测速接口不受带宽、请求频率和并发请求数限制的约束。

## `global` 与配额组

配额有两个作用范围。`global` 对所有站点生效。配额组是一组共享配额的站点，在 `groups` 中定义，站点通过 `quota_group` 加入其中一个。

**`global` 和配额组的配额同时生效，请求必须满足两者的全部限制。** 所以配额组只能在全局之上进一步收紧，不能放宽。想给不同站点设置不同的配额，就把它们分到不同的组，`global` 只放整个实例都要遵守的上限。

下面的配置让普通站点的每个客户端最多同时处理 4 个请求，下载站点放宽到 12 个，所有客户端合计带宽不超过 100 MiB/s：

```yaml
resource_limit:
  global:
    all_clients:
      traffic_avg_mibps: 100
  groups:
    - id: general
      per_client:
        request_concurrency: 4
    - id: downloads
      per_client:
        request_concurrency: 12

sites:
  - host: pypi.example.com
    mode: pypi
    quota_group: general
  - host: gh.example.com
    mode: github
    quota_group: downloads
  - host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
    quota_group: downloads
  # 其他 host 不受 downloads 配额组的影响
```

GitHub 和 Hugging Face 共享 `downloads` 组的配额，同一个客户端在这两个站点上合计最多 12 个并发请求。如果把 `request_concurrency: 4` 写在 `global` 中，下载站点也会受到这个 4 的限制。

## `per_client` 与 `all_clients`

在每个作用范围内，`per_client` 中的限制由每个客户端各自持有一份配额，`all_clients` 中的限制由所有客户端共用一份配额。两者可配置的字段相同，可以同时配置。

客户端按来源 IP 划分：IPv4 按单个地址，IPv6 按所在的 `/64` 网段。同一 `/64` 网段内的 IPv6 地址共享同一份配额。位于反向代理之后时，客户端 IP 取决于 [`server`](../config/server) 中受信任代理的配置。

## 各项限制

| 限制 | 字段 | 超出限制时 |
|------|------|------------|
| 带宽 | `traffic_*` | 传输降速，请求不会被中断 |
| 请求频率 | `request_per_second` / `minute` / `hour` | 返回 `429 Too Many Requests` |
| 并发请求数 | `request_concurrency` | 返回 `429 Too Many Requests` |
| 请求超时 | `request_timeout` | 终止请求 |

### 带宽

使用令牌桶算法控制流量速率：桶以 `traffic_avg_mibps` 的速率持续填充令牌，每传输 1 MiB 数据消耗 1 个令牌。空闲时积累的令牌（最多 `traffic_burst_mib` 个）允许短时间内超过平均速率。`traffic_max_mibps` 则限制峰值速率。

```text
           traffic_avg_mibps（持续补充）
                  │
                  ▼
           ┌─────────────┐
           │ ● ● ● ● ●   │  容量 traffic_burst_mib
           │ ● ● ● ● ●   │  空闲时攒满，可短时突发
           └──────┬──────┘
                  │ 每 1 MiB 取走 1 个令牌
                  ▼
       传输速率 ≤ traffic_max_mibps
```

流量按上传与下载合计计算。

### 请求频率

限制客户端发往 Pavonis 的请求数。秒、分钟、小时三个粒度可同时生效，超出任意一个即拒绝请求。

### 并发请求数

限制同时处理的请求数。下载进行中的请求一直占用名额，传输结束后才释放。名额已满时，新请求立即被拒绝，不会排队等待。

### 请求超时

单个请求的最长处理时间，全局默认 `1h`。站点所在的配额组也配置了超时时，取两者中较短的一个。
