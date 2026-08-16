---
title: RequestConfig
order: 2
---

# RequestConfig

The `request` section configures how Pavonis sends outbound requests to upstreams, including the outbound proxy, IP pool and request header modification.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `proxy` | `string` | none | Outbound HTTP proxy address, e.g. `http://127.0.0.1:7890`. All upstream requests will go through this proxy |
| `ip_pool` | `IpPoolConfig` | see below | IP pool config, [see below](#ipoolconfig-ip-pool) |
| `header` | `HeaderModificationConfig` | see below | Outbound request header modification config, [see below](#headermodificationconfig-request-headers) |

---

## IpPoolConfig (IP Pool)

Allows Pavonis to select outbound IP addresses from specified IPv6 subnets.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `false` | Whether to enable the IP pool. `subnets` must not be empty when enabled |
| `default_strategy` | `IpPoolStrategy` | `none` | Default outbound IP selection strategy; can be overridden per site via `ip_pool_strategy` |
| `subnets` | `[]string` | `[]` | CIDR subnet list for outbound IPs |

**Strategy enum values:**

| Value | Description |
|-------|-------------|
| `none` | Do not use the IP pool; use the system default outbound IP |
| `random` | Randomly select an IP from the pool for each request |
| `ip_hash` | Hash the client IP; the same client always uses the same outbound IP |

---

## HeaderModificationConfig (Request Headers)

Controls the headers sent with upstream requests.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `modify` | `map[string]string` | `{}` | Add or overwrite specified request headers |
| `delete` | `[]string` | see below | Delete specified request headers |

**Default headers removed by `header.delete` (prevents proxy information leaking to upstreams):**

```yaml
# Common reverse proxy headers
- 'Via'
- 'X-Forwarded-For'
- 'X-Forwarded-Proto'
- 'X-Forwarded-Host'
# Cloudflare-related headers
- 'CDN-Loop'
- 'CF-Connecting-IP'
- 'CF-Connecting-IPv6'
- 'CF-EW-Via'
- 'CF-IPCountry'
- 'CF-Pseudo-IPv4'
- 'Cf-Ray'
- 'CF-Visitor'
- 'Cf-Warp-Tag-Id'
```

::: warning
Explicitly configuring `header.delete` **completely replaces** the default list rather than appending to it. To keep the default removal behaviour and also delete additional headers, you must include the default list manually.
:::

## Configuration Examples

### Use an Outbound Proxy

```yaml
request:
  proxy: http://127.0.0.1:7890
```

### Configure IP Pool (Random Strategy)

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: random
    subnets:
      - '2001:db8::/48'
```

### Configure IP Pool (Different Strategy per Site)

```yaml
request:
  ip_pool:
    enabled: true
    default_strategy: none     # no IP pool by default
    subnets:
      - '2001:db8::/48'

sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    ip_pool_strategy: ip_hash  # this site uses ip_hash
```

### Add Custom Request Headers to Upstream

```yaml
request:
  header:
    modify:
      X-Custom-Header: 'my-value'
      Authorization: 'Bearer secret-token'
```

### Customise the Delete Header List

```yaml
request:
  header:
    delete:
      - 'Via'
      - 'X-Forwarded-For'
      - 'X-Forwarded-Proto'
      - 'X-Forwarded-Host'
      - 'CDN-Loop'
      - 'CF-Connecting-IP'
      - 'CF-Connecting-IPv6'
      - 'CF-EW-Via'
      - 'CF-IPCountry'
      - 'CF-Pseudo-IPv4'
      - 'Cf-Ray'
      - 'CF-Visitor'
      - 'Cf-Warp-Tag-Id'
      - 'X-My-Internal-Header'  # also remove this custom header
```
