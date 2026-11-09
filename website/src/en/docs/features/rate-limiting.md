---
title: Traffic and Request Rate Limiting
order: 2
---

# Traffic and Request Rate Limiting

Pavonis supports two types of rate limiting: traffic limiting (bandwidth control) and request rate limiting (QPS control). Both are configured under [`ResourceLimitConfig`](../config/resource-limit).

Limits apply at two levels:

- Per client: traffic and request rate limits are tracked separately for each client, and each client has its own quota
- Global: traffic can additionally be capped for all clients combined

A request must satisfy all configured limits.

## What Counts as a Client

Clients are identified by source IP: IPv4 by individual address, IPv6 by its `/64` subnet. IPv6 addresses within the same `/64` subnet share one quota.

When Pavonis runs behind a reverse proxy, the client IP depends on the trusted proxy settings in [`server`](../config/server).

## Traffic Limiting

Uses a token bucket algorithm to control the traffic rate: the bucket fills at `traffic_avg_mibps` tokens per second, and transferring 1 MiB of data consumes 1 token. The bucket holds up to `traffic_burst_mib` tokens; tokens accumulated while idle allow short bursts above the average rate.

Traffic is counted as upload and download combined; both directions share the same quota.

### Per-Client Traffic Limiting

| Field | Description |
|-------|-------------|
| `traffic_avg_mibps` | Long-term average rate (token fill rate, in MiB/s) |
| `traffic_burst_mib` | Burst capacity (bucket size, in MiB) |
| `traffic_max_mibps` | Peak rate cap (in MiB/s) |

```yaml
resource_limit:
  traffic_avg_mibps: 10    # average 10 MiB/s per client
  traffic_burst_mib: 20    # allow each client to burst 20 MiB
  traffic_max_mibps: 30    # each client never exceeds 30 MiB/s
```

### Global Traffic Limiting

Caps the combined traffic of all clients, useful for controlling the server's total bandwidth. The fields mirror the per-client ones:

| Field | Description |
|-------|-------------|
| `global_traffic_avg_mibps` | Long-term average rate of all clients combined (MiB/s) |
| `global_traffic_burst_mib` | Burst capacity of all clients combined (MiB) |
| `global_traffic_max_mibps` | Peak rate cap of all clients combined (MiB/s) |

```yaml
resource_limit:
  traffic_avg_mibps: 10           # average 10 MiB/s per client
  global_traffic_avg_mibps: 100   # average 100 MiB/s across all clients
  global_traffic_burst_mib: 50
```

## Request Rate Limiting

Limits the number of requests per client. Supports per-second, per-minute and per-hour granularities simultaneously:

| Field | Description |
|-------|-------------|
| `request_per_second` | Maximum requests per second per client |
| `request_per_minute` | Maximum requests per minute per client |
| `request_per_hour` | Maximum requests per hour per client |

```yaml
resource_limit:
  request_per_second: 50
  request_per_minute: 1000
  request_per_hour: 10000
```

A request exceeding any of these limits is rejected with `429 Too Many Requests`.

## Scope

All fields are optional. Omitting a field means no limit on that dimension.

These limits apply to all proxy sites. [Speed test](../sites/speed-test) sites are not rate limited.

## Reference

- [`ResourceLimitConfig` full documentation](../config/resource-limit)
