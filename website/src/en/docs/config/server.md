---
title: ServerConfig
order: 1
---

# ServerConfig

The `server` section configures the listen address and the trusted front proxies (e.g. Nginx, a CDN) that sit between clients and Pavonis.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen` | `string` | `:8009` | Listen address in `host:port` format. `0.0.0.0:8009` listens on all interfaces; `:8009` is equivalent |
| `trusted_proxy_ips` | `[]string` | `["127.0.0.1/24"]` | IP list of trusted front proxies (CIDR supported). Proxy headers from these addresses are trusted. Use `["*"]` to trust all sources |
| `trusted_proxy_headers` | `[]string` | `[]` | Which request headers to read the real client IP from; the first usable one wins. Empty by default, i.e. no header is read |

## Getting the Real Client IP

Behind a front proxy, Pavonis sees the connection coming from the proxy. Configure `trusted_proxy_headers` so that Pavonis can read the real client IP from request headers. The client IP is used for logging, the `ip_hash` strategy and per-client rate limiting.

Only list headers that your front proxy sets on every request. A header the front proxy does not touch reaches Pavonis as-is, and the client can forge it freely. For example, behind Nginx a client can send its own `CF-Connecting-IP`.

`X-Forwarded-For` may contain multiple IPs. Pavonis reads it from right to left, skips addresses in `trusted_proxy_ips`, and takes the first untrusted IP. With multiple layers of front proxies, add the address of every layer to `trusted_proxy_ips`.

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

### Behind Nginx

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Real-IP'
```

Nginx must set the header:

```nginx
proxy_set_header X-Real-IP $remote_addr;
```

### Behind Caddy, Traefik or Other Proxies That Append X-Forwarded-For

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Forwarded-For'
```

## Notes

- If Pavonis is exposed directly to the internet, avoid setting `trusted_proxy_ips` to `["*"]`, as any client could then forge the headers in `trusted_proxy_headers`.
