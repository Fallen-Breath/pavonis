---
title: ResourceLimitConfig
order: 4
---

# ResourceLimitConfig

The `resource_limit` section configures bandwidth, request rate, concurrent requests and the request timeout. The quota in `global` applies to all sites, and the quota of a quota group applies only to the sites in that group. Both constrain a request together. See [Resource Limits](../features/rate-limiting) for how the limits work.

All fields are optional. Within a quota, the request timeout defaults to `1h` and other limits are unlimited when omitted.

## ResourceLimitConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `global` | [`ResourceQuotaConfig`](#resourcequotaconfig-quota) | `1h` request timeout | Quota applied to all sites |
| `groups` | [`[]QuotaGroupConfig`](#quotagroupconfig-quota-group) | None | Quota groups. A site joins one of them with [`quota_group`](./sites) |
| `max_concurrent_429_responses` | `int` | `64` | Maximum number of `429` rejection responses being sent at once. Beyond that, newly rejected requests are aborted without a response |
| `idle_client_cache_size` | `int` | `10240` | How many idle `per_client` usages are kept. Beyond that, the least recently seen ones are reset |

## ResourceQuotaConfig (Quota)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_timeout` | `duration` | `1h` in `global`, no extra limit in a group | Maximum processing time of a single request, in Go duration format such as `30m`, `2h` or `90s`. When set in a group, the shorter of it and the global timeout applies |
| `per_client` | [`ClientQuotaConfig`](#clientquotaconfig-limits) | No limit | Limits with a separate quota for each client |
| `all_clients` | [`ClientQuotaConfig`](#clientquotaconfig-limits) | No limit | Limits with one quota shared by all clients |

## QuotaGroupConfig (Quota Group)

A quota group has all fields of `ResourceQuotaConfig`, plus:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | **Yes** | Group name, referenced by a site's `quota_group` |

## ClientQuotaConfig (Limits)

### Bandwidth (Token Bucket)

Traffic is counted as upload and download combined.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `traffic_avg_mibps` | `float` | No limit | Average rate (MiB/s), i.e. the token bucket refill rate |
| `traffic_burst_mib` | `float` | See below | Burst capacity (MiB), i.e. the token bucket size |
| `traffic_max_mibps` | `float` | No limit | Peak rate cap (MiB/s) |

When `traffic_burst_mib` is not set, it defaults to the smaller of `traffic_avg_mibps` and `0.125`. It only takes effect when `traffic_avg_mibps` is set.

### Requests

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_per_second` | `float` | No limit | Maximum requests per second |
| `request_per_minute` | `float` | No limit | Maximum requests per minute |
| `request_per_hour` | `float` | No limit | Maximum requests per hour |
| `request_concurrency` | `int` | No limit | Maximum requests processed at the same time |

A request exceeding any of them is rejected with `429 Too Many Requests`.

## Configuration Examples

### Limit Per-Client Bandwidth

```yaml
resource_limit:
  global:
    per_client:
      traffic_avg_mibps: 10      # 10 MiB/s average per client
      traffic_burst_mib: 20      # burst up to 20 MiB per client
      traffic_max_mibps: 30      # peak never exceeds 30 MiB/s per client, even when bursting
```

### Limit Total Server Bandwidth

```yaml
resource_limit:
  global:
    per_client:
      traffic_avg_mibps: 10      # 10 MiB/s average per client
    all_clients:
      traffic_avg_mibps: 100     # 100 MiB/s average for all clients combined
      traffic_burst_mib: 50
```

### Limit Request Rate and Concurrency

```yaml
resource_limit:
  global:
    per_client:
      request_per_second: 20
      request_per_minute: 500
      request_concurrency: 8
```

### Set Request Timeout

```yaml
resource_limit:
  global:
    request_timeout: 30m    # requests exceeding 30 minutes will be terminated
```

### Separate Quota for Some Sites

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
    quota_group: downloads
  - host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
    quota_group: downloads
  # other hosts are not affected by the downloads quota group
```

GitHub and Hugging Face together process at most 50 requests at once within 200 MiB/s of total bandwidth, and each client is still limited to 20 requests per second by `global`.
