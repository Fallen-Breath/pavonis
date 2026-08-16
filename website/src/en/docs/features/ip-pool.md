---
title: Outbound IP Pool
order: 1
---

# Outbound IP Pool

## Concept

By default Pavonis uses the system outbound IP for all upstream requests. When you hold a large IPv6 address block (e.g. a `/48` subnet from HE Tunnel Broker), you can enable the **IP pool** to let Pavonis select an outbound IP from the pool according to a strategy, bypassing per-IP rate limits or access blocks imposed by upstream platforms.

## Configuration

The IP pool is configured under the `ip_pool` field of [`RequestConfig`](../config/request):

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: random
    subnets:
      - '2001:db8::/48'
      - '2001:db8:1::/64'
```

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | `bool` | Whether to enable the IP pool |
| `default_strategy` | [`IpPoolStrategy`](../config/request) | Global default outbound strategy |
| `subnets` | `[]string` | List of available IPv6 subnets in CIDR notation |

## Outbound Strategies

| Strategy | Description |
|----------|-------------|
| `none` | Do not use the IP pool; use the system default outbound IP (default) |
| `random` | Randomly pick an IP from the pool for each request |
| `ip_hash` | Hash the client IP; the same client always uses the same outbound IP |

`random` is best when you want to spread requests across as many IPs as possible. `ip_hash` suits scenarios where the upstream needs to recognise the "same user" (e.g. APIs requiring session affinity).

## Per-Site Override

The global `default_strategy` can be overridden by a site's `ip_pool_strategy` field for fine-grained control:

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: none       # most sites don't use the IP pool by default
    subnets:
      - '2001:db8::/48'

sites:
  - host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
    ip_pool_strategy: random     # this site uses a random outbound IP

  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    ip_pool_strategy: ip_hash    # this site pins outbound IP by client
```

## Reference

- [`RequestConfig` full documentation](../config/request)
- [`SiteConfig` site-level configuration](../config/sites)
