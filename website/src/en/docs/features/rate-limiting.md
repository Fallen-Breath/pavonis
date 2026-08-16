---
title: Traffic and Request Rate Limiting
order: 2
---

# Traffic and Request Rate Limiting

Pavonis supports two types of rate limiting: **outbound traffic limiting** (bandwidth control) and **request rate limiting** (QPS control). Both are configured globally under [`ResourceLimitConfig`](../config/resource-limit) and take effect independently — either can trigger a rejection.

## Outbound Traffic Limiting

Uses a **token bucket** algorithm to control outbound traffic rate. How it works: the bucket fills at `traffic_avg_mibps` tokens per second; sending 1 MiB of data consumes 1 token; excess tokens are discarded when the bucket is full, with a maximum capacity of `traffic_burst_mib`.

| Field | Description |
|-------|-------------|
| `traffic_avg_mibps` | Long-term average rate (token fill rate, in MiB/s) |
| `traffic_burst_mib` | Allowed burst capacity (bucket size, in MiB) |
| `traffic_max_mibps` | Peak rate cap (in MiB/s); unlimited if not set |

```yaml
resource_limit:
  traffic_avg_mibps: 100   # average 100 MiB/s
  traffic_burst_mib: 200   # allow bursting up to 200 MiB
  traffic_max_mibps: 300   # peak never exceeds 300 MiB/s
```

All three fields are optional. Omitting a field means no limit on that dimension.

## Request Rate Limiting

Limits the total number of inbound requests globally. Supports **per-second, per-minute and per-hour** granularities simultaneously:

| Field | Description |
|-------|-------------|
| `request_per_second` | Maximum requests per second |
| `request_per_minute` | Maximum requests per minute |
| `request_per_hour` | Maximum requests per hour |

```yaml
resource_limit:
  request_per_second: 50
  request_per_minute: 1000
  request_per_hour: 10000
```

All conditions are enforced simultaneously. A request is rejected if any condition is exceeded. Omitting a field means no limit on that dimension.

## Reference

- [`ResourceLimitConfig` full documentation](../config/resource-limit)
