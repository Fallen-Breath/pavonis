---
title: HTTPS and Reverse Proxy
order: 2
---

# HTTPS and Reverse Proxy

Pavonis only serves plain HTTP and does not handle TLS. To serve your sites over HTTPS, put a reverse proxy in front of Pavonis to terminate TLS and forward requests to the `server.listen` address. Set `self_url` to the public HTTPS address that clients use.

Pavonis routes requests to sites by their `Host` header, so the front proxy must forward the client's original `Host`.

The front proxy should also pass on the real client IP, which Pavonis uses for logging, per-client rate limiting and the `ip_hash` strategy. Each example below pairs the proxy config with the matching [`server`](../config/server) config. The examples assume the proxy and Pavonis run on the same host. If Pavonis runs in Docker, listen on `0.0.0.0:8009` and put the address the proxy connects from (e.g. the Docker network gateway) in `trusted_proxy_ips`.

## Caddy

Caddy obtains certificates automatically and keeps `Host` by default, so it is the simplest option.

```text
docker.example.com, gh.example.com {
    reverse_proxy 127.0.0.1:8009
}
```

Caddy appends the client IP to `X-Forwarded-For`:

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Forwarded-For'
```

## Nginx

Nginx rewrites `Host` to the upstream address by default, so `proxy_set_header Host` is required. Certificates are configured as usual and omitted here.

```nginx
server {
    listen 443 ssl;
    server_name docker.example.com gh.example.com;

    location / {
        proxy_pass http://127.0.0.1:8009;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Real-IP'
```

## Cloudflare Tunnel

Cloudflare terminates TLS and `cloudflared` keeps `Host` by default. The origin needs no open ports.

```yaml
ingress:
  - hostname: docker.example.com
    service: http://127.0.0.1:8009
  - hostname: gh.example.com
    service: http://127.0.0.1:8009
  - service: http_status:404
```

Cloudflare puts the client IP in `CF-Connecting-IP`. Since Pavonis only accepts connections from the local `cloudflared`, trusting `127.0.0.1` is enough:

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'CF-Connecting-IP'
```
