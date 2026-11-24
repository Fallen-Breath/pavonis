---
title: HTTPS and Reverse Proxy
order: 2
---

# HTTPS and Reverse Proxy

There are two ways to serve your sites over HTTPS: configure `server.tls` so Pavonis serves HTTPS directly, or put a reverse proxy in front of Pavonis to handle HTTPS and forward HTTP requests to `server.listen`. Either way, set `self_url` to the public HTTPS address that clients use.

## Direct HTTPS

When you already have certificate files and Pavonis can bind the port directly, configure `server.tls` to serve HTTPS without a front proxy. Updated certificate files are reloaded automatically:

```yaml
server:
  listen: 0.0.0.0:443
  tls:
    certificates:
      - cert_file: /etc/pavonis/certs/fullchain.pem
        key_file: /etc/pavonis/certs/privkey.pem
```

See [`ServerConfig`](../config/server#servertlsconfig) for field details and multi-certificate matching rules. Pavonis does not obtain certificates automatically or listen on an extra port to redirect HTTP to HTTPS. Use a front reverse proxy if you need automatic certificate management, share port `443` with other services on the same host, or keep no ports open on the origin.

## Behind a Reverse Proxy

Pavonis routes requests to sites by their `Host` header, so the front proxy must forward the client's original `Host`.

The front proxy should also pass on the real client IP, which Pavonis uses for logging, `per_client` resource limits and the `ip_hash` strategy. Each example below pairs the proxy config with the matching [`server`](../config/server) config. The examples assume the proxy and Pavonis run on the same host. If Pavonis runs in Docker, listen on `0.0.0.0:8009` and put the address the proxy connects from (e.g. the Docker network gateway) in `trusted_proxy_ips`.

### Caddy

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

### Nginx

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

### Cloudflare Tunnel

Cloudflare handles HTTPS and `cloudflared` keeps `Host` by default. The origin needs no open ports.

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
