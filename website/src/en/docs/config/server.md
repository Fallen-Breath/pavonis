---
title: ServerConfig
order: 1
---

# ServerConfig

The `server` section configures the listen address and trusted upstream proxies.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen` | `string` | `:8009` | Listen address in `host:port` format. `0.0.0.0:8009` listens on all interfaces; `:8009` is equivalent |
| `trusted_proxy_ips` | `[]string` | `["127.0.0.1/24"]` | Trusted upstream proxy IP list (CIDR supported). Proxy headers from these addresses are trusted. Use `["*"]` to trust all sources |
| `trusted_proxy_headers` | `[]string` | see below | Which request headers to read the real client IP from |

**Default value of `trusted_proxy_headers`:**

```yaml
- 'CF-Connecting-IP'   # Cloudflare real IP
- 'X-Forwarded-For'    # standard proxy header
- 'X-Real-IP'          # common alternative
```

## Configuration Examples

### Minimal Config

```yaml
server:
  listen: 0.0.0.0:8009
```

### Behind Cloudflare

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '*'              # trust all sources (Cloudflare has many exit IPs)
  trusted_proxy_headers:
    - 'CF-Connecting-IP'
```

### Behind an Internal Nginx

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
    - '192.168.1.0/24'
  trusted_proxy_headers:
    - 'X-Real-IP'
```

## Notes

- `trusted_proxy_ips` tells Pavonis to trust proxy headers from the specified IPs, enabling it to obtain the real client IP (primarily used for logging and the `ip_hash` strategy).
- If Pavonis is exposed directly to the internet, avoid setting `trusted_proxy_ips` to `["*"]`, as any client could then forge `X-Forwarded-For` and similar headers.
