---
title: Traffic and Request Rate Limiting
order: 2
---

# Traffic and Request Rate Limiting

Pavonis supports two types of rate limiting: traffic limiting (bandwidth control) and request rate limiting (QPS control). All limits are optional. See [`ResourceLimitConfig`](../config/resource-limit) for the fields and configuration examples.

Limits apply at two levels:

- Per client: traffic and request rate limits are tracked separately for each client, and each client has its own quota
- Global: traffic can additionally be capped for all clients combined

```text
request ──► client request rate (s/min/h) ── exceeded ──► 429
              │
              ▼ allowed
data ─────► client traffic bucket ──► global traffic bucket ──► transfer
            (waits when short of tokens: slows down, never rejects)
```

A request must satisfy all configured limits. The limits apply to all proxy sites except [speed test](../sites/speed-test) sites.

## What Counts as a Client

Clients are identified by source IP: IPv4 by individual address, IPv6 by its `/64` subnet. IPv6 addresses within the same `/64` subnet share one quota.

When Pavonis runs behind a reverse proxy, the client IP depends on the trusted proxy settings in [`server`](../config/server).

## Traffic Limiting

Uses a token bucket algorithm to control the traffic rate: the bucket fills at `traffic_avg_mibps` tokens per second, and transferring 1 MiB of data consumes 1 token. Tokens saved up while idle, up to `traffic_burst_mib`, allow short bursts above the average rate. `traffic_max_mibps` caps the peak rate.

```text
           traffic_avg_mibps (steady refill)
                  │
                  ▼
           ┌─────────────┐
           │ ● ● ● ● ●   │  capacity traffic_burst_mib
           │ ● ● ● ● ●   │  fills up while idle, allows short bursts
           └──────┬──────┘
                  │ 1 token per 1 MiB transferred
                  ▼
       transfer rate ≤ traffic_max_mibps
```

When traffic exceeds the limit, the transfer waits for tokens: it slows down rather than being interrupted or rejected.

Traffic is counted as upload and download combined. The `global_traffic_*` fields mirror the per-client ones and apply to the combined traffic of all clients, useful for controlling the server's total bandwidth.

## Request Rate Limiting

Limits the number of requests per client. Per-second, per-minute and per-hour limits can apply simultaneously. A request exceeding any of them is rejected with `429 Too Many Requests`.
