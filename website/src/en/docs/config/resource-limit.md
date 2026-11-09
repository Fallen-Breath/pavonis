---
title: ResourceLimitConfig
order: 4
---

# ResourceLimitConfig

The `resource_limit` section configures traffic rate limits, request frequency limits and the request timeout, preventing the proxy from consuming excessive bandwidth or being overwhelmed.

Traffic and request frequency limits are tracked separately for each client (IPv4 by individual address, IPv6 by `/64` subnet); in addition, a global cap on the combined traffic of all clients can be configured. See [Traffic and Request Rate Limiting](../features/rate-limiting) for how the limits work.

All rate limit fields are optional. Omitting a field means no limit on that dimension. Configured values must be greater than 0.

## Fields

### Traffic Rate Limiting (Token Bucket)

Per-client traffic limits. Traffic is counted as upload and download combined.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `traffic_avg_mibps` | `float` | unlimited | Average rate per client (MiB/s) — the token bucket fill rate |
| `traffic_burst_mib` | `float` | see below | Burst capacity per client (MiB) — the token bucket size |
| `traffic_max_mibps` | `float` | unlimited | Peak rate cap per client (MiB/s) |

- `traffic_avg_mibps` controls the long-term average rate (token fill rate)
- `traffic_burst_mib` controls the allowed instantaneous burst (bucket size); tokens accumulate during idle periods, allowing short-term spikes above the average. Defaults to the smaller of `traffic_avg_mibps` and `0.125`; only takes effect when `traffic_avg_mibps` is set
- `traffic_max_mibps` can additionally cap the peak rate

### Global Traffic Rate Limiting

Traffic limits on all clients combined. Each field has the same meaning and default rules as its per-client counterpart above.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `global_traffic_avg_mibps` | `float` | unlimited | Average rate of all clients combined (MiB/s) |
| `global_traffic_burst_mib` | `float` | see above | Burst capacity of all clients combined (MiB) |
| `global_traffic_max_mibps` | `float` | unlimited | Peak rate cap of all clients combined (MiB/s) |

### Request Rate Limiting

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_per_second` | `float` | unlimited | Maximum requests per second per client |
| `request_per_minute` | `float` | unlimited | Maximum requests per minute per client |
| `request_per_hour` | `float` | unlimited | Maximum requests per hour per client |

Multiple request rate limits can be configured simultaneously; a request must satisfy all conditions to be processed, otherwise `429 Too Many Requests` is returned.

### Timeout

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_timeout` | `duration` | `1h` | Maximum processing time for a single request. Uses Go duration format, e.g. `30m`, `2h`, `90s` |

## Configuration Examples

### Limit Per-Client Bandwidth

```yaml
resource_limit:
  traffic_avg_mibps: 10      # average 10 MiB/s per client
  traffic_burst_mib: 20      # allow each client to burst up to 20 MiB
```

### Full Traffic Rate Limiting

```yaml
resource_limit:
  traffic_avg_mibps: 10
  traffic_burst_mib: 20
  traffic_max_mibps: 30      # each client never exceeds 30 MiB/s even with burst
```

### Limit Total Server Bandwidth

```yaml
resource_limit:
  traffic_avg_mibps: 10            # average 10 MiB/s per client
  global_traffic_avg_mibps: 100    # average 100 MiB/s across all clients
  global_traffic_burst_mib: 50
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
