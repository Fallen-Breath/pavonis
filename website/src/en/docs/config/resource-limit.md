---
title: ResourceLimitConfig
order: 4
---

# ResourceLimitConfig

The `resource_limit` section configures global traffic rate and request frequency limits to prevent the proxy from consuming excessive bandwidth or being overwhelmed.

**All fields are optional. Omitting a field means no limit on that dimension. Values must be greater than 0 (or `-1` to disable, applicable to speed-test-related fields only).**

## Fields

### Traffic Rate Limiting (Token Bucket)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `traffic_avg_mibps` | `float` | unlimited | Average outbound traffic rate (MiB/s) — the token bucket fill rate |
| `traffic_burst_mib` | `float` | unlimited | Outbound traffic burst capacity (MiB) — the token bucket size |
| `traffic_max_mibps` | `float` | unlimited | Peak outbound traffic rate cap (MiB/s) |

Token bucket behaviour:
- `traffic_avg_mibps` controls the long-term average rate (token fill rate)
- `traffic_burst_mib` controls the allowed instantaneous burst (bucket size); tokens accumulate during idle periods, allowing short-term spikes above the average
- `traffic_max_mibps` can additionally cap the peak rate

### Request Rate Limiting

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_per_second` | `float` | unlimited | Maximum global requests per second |
| `request_per_minute` | `float` | unlimited | Maximum global requests per minute |
| `request_per_hour` | `float` | unlimited | Maximum global requests per hour |

Multiple request rate limits can be configured simultaneously; a request must satisfy all conditions to be processed.

### Timeout

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_timeout` | `duration` | `1h` | Maximum processing time for a single request. Uses Go duration format, e.g. `30m`, `2h`, `90s` |

## Configuration Examples

### Limit Outbound Bandwidth

```yaml
resource_limit:
  traffic_avg_mibps: 100     # average 100 MiB/s
  traffic_burst_mib: 200     # allow bursting up to 200 MiB
```

### Full Traffic Rate Limiting

```yaml
resource_limit:
  traffic_avg_mibps: 50
  traffic_burst_mib: 100
  traffic_max_mibps: 200     # peak never exceeds 200 MiB/s even with burst
```

### Limit Request Frequency

```yaml
resource_limit:
  request_per_second: 20
  request_per_minute: 500
  request_per_hour: 5000
```

### Set Request Timeout

```yaml
resource_limit:
  request_timeout: 30m    # requests exceeding 30 minutes are terminated
```

### Combined Example

```yaml
resource_limit:
  traffic_avg_mibps: 100
  traffic_burst_mib: 50
  request_per_second: 50
  request_timeout: 1h
```
