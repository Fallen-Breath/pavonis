---
title: Resource Limits
order: 2
---

# Resource Limits

Resource limits control bandwidth, request rate, concurrent requests and the request timeout, keeping the proxy from consuming excessive resources or being abused. All limits are optional. See [`ResourceLimitConfig`](../config/resource-limit) for the fields and configuration examples. The test endpoints of [speed test](../sites/speed-test) sites are not subject to the bandwidth, request rate or concurrency limits.

## `global` and Quota Groups

Quotas have two scopes. `global` applies to all sites. A quota group is a set of sites sharing one quota. Groups are defined in `groups`, and a site joins one of them with `quota_group`.

**The quotas of `global` and of the group apply together: a request must satisfy all limits of both.** So a group can only tighten the global limits, never relax them. To give different sites different quotas, put them in different groups and keep only the limits the whole instance must obey in `global`.

The configuration below allows each client up to 4 concurrent requests on general sites and up to 12 on download sites, while all clients combined stay within 100 MiB/s of bandwidth:

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
  # other hosts are not affected by the downloads quota group
```

GitHub and Hugging Face share the quota of the `downloads` group, so one client can have at most 12 concurrent requests across the two sites. Putting `request_concurrency: 4` in `global` would limit the download sites to 4 as well.

## `per_client` and `all_clients`

Within each scope, limits under `per_client` give every client a quota of its own, and limits under `all_clients` are one quota shared by all clients. Both accept the same fields and can be configured together.

Clients are identified by source IP: IPv4 by individual address, IPv6 by its `/64` subnet. IPv6 addresses within the same `/64` subnet share one quota. When Pavonis runs behind a reverse proxy, the client IP depends on the trusted proxy settings in [`server`](../config/server).

## Limits

| Limit | Fields | When exceeded |
|-------|--------|---------------|
| Bandwidth | `traffic_*` | The transfer slows down, the request is not interrupted |
| Request rate | `request_per_second` / `minute` / `hour` | `429 Too Many Requests` |
| Concurrent requests | `request_concurrency` | `429 Too Many Requests` |
| Request timeout | `request_timeout` | The request is terminated |

### Bandwidth

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

Traffic is counted as upload and download combined.

### Request Rate

Limits the number of requests a client sends to Pavonis. Per-second, per-minute and per-hour limits can apply simultaneously, and a request exceeding any of them is rejected.

### Concurrent Requests

Limits the number of requests being processed at the same time. A request keeps its slot while its download is in progress and frees it when the transfer ends. When all slots are taken, a new request is rejected immediately instead of waiting in a queue.

### Request Timeout

The maximum processing time of a single request, `1h` by default. If the site's quota group also sets a timeout, the shorter of the two applies.
