---
title: RequestConfig
order: 2
---

# RequestConfig

The `request` section configures how Pavonis sends outbound requests to upstreams, including the request proxy, IP pool and request header modification.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `proxy` | `RequestProxyConfig` | see below | Proxy config for upstream requests. Without any `servers`, upstream requests connect directly. [See below](#requestproxyconfig) |
| `ip_pool` | `IpPoolConfig` | see below | IP pool config, [see below](#ippoolconfig-ip-pool) |
| `header` | `HeaderModificationConfig` | see below | Outbound request header modification config, [see below](#headermodificationconfig-request-headers) |

---

## RequestProxyConfig

Routes upstream requests through proxy servers. With several proxies configured, requests use the first available proxy in the configured order.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `servers` | `[]RequestProxyServerConfig` | `[]` | Proxy servers in priority order |
| `direct_fallback` | `bool` | `false` | Connect directly when all proxies are unavailable |
| `connect_timeout` | `duration` | `5s` | Timeout for connecting to a proxy |
| `block` | `RequestProxyBlockConfig` | see below | Rules for temporarily skipping unavailable proxies |

**RequestProxyServerConfig**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `url` | `string` | none | Proxy address, e.g. `http://127.0.0.1:7890`. The scheme must be one of `http`, `https`, `socks5`, `socks5h`. Credentials can be given as `http://user:pass@host:port` |
| `proxy_protocol` | `ProxyProtocol` | `none` | PROXY protocol version sent when connecting to this proxy, to pass the client IP. Options: `none` (disabled), `v1`, `v2`. The port of the proxy must have the corresponding version enabled. Never sent on direct connections |

**RequestProxyBlockConfig**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `failure_threshold` | `int` | `3` | Number of consecutive connection failures after which a proxy is blocked |
| `cooldown` | `duration` | `10s` | How long a proxy is blocked for the first time |
| `max_cooldown` | `duration` | `5m` | Upper bound of the blocking time |

**Switching rules**

A request uses the first proxy in `servers` that is not blocked. After `failure_threshold` consecutive connection failures, a proxy is blocked and skipped. When the block ends the proxy is tried again, and each further failure doubles the blocking time, up to `max_cooldown`. When all proxies are blocked, requests keep going through them unless `direct_fallback` is enabled, in which case they connect directly.

When connecting to a proxy fails, a request without a body is sent again via the next proxy, and finally directly if `direct_fallback` is enabled. A request with a body returns an error.

Only failing to connect to a proxy counts as a failure, including exceeding `connect_timeout`. Errors returned by a proxy after the connection is established, such as `407`, `502` or a rejected `CONNECT`, do not affect its availability.

---

## IpPoolConfig (IP Pool)

Allows Pavonis to select outbound IP addresses from the specified subnets.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `false` | Whether to enable the IP pool |
| `default_strategy` | `IpPoolStrategy` | `none` | Default outbound IP strategy, overridable per site with `ip_pool_strategy` |
| `subnets` | `[]string` | `[]` | CIDR subnet list for outbound IPs |

**Strategy enum values:**

| Value | Description |
|-------|-------------|
| `none` | Use the system default outbound IP |
| `random` | Randomly select an IP from the pool for each request |
| `ip_hash` | Pin each client to one outbound IP |

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

### Use a Request Proxy

```yaml
request:
  proxy:
    servers:
      - url: http://127.0.0.1:7890
```

To pass the client IP to the proxy, set `proxy_protocol`. The proxy's receiving port must support and have the corresponding PROXY protocol version enabled.

```yaml
request:
  proxy:
    servers:
      - url: http://127.0.0.1:7890
        proxy_protocol: v2
```

Clients are identified by IP as configured in [ServerConfig](./server).

### Use Multiple Request Proxies, Falling Back to Direct

```yaml
request:
  proxy:
    servers:
      - url: http://10.0.0.1:7890
        proxy_protocol: v2
      - url: socks5://10.0.0.2:1080
    direct_fallback: true
```

Priority order: `10.0.0.1`, `10.0.0.2`, then a direct connection.

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
