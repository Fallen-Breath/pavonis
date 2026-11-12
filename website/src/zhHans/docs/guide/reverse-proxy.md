---
title: HTTPS 与反向代理
order: 2
---

# HTTPS 与反向代理

Pavonis 只提供明文 HTTP，不处理 TLS。要对外提供 HTTPS，需要在 Pavonis 前面配置反向代理，由它终结 TLS，再将请求转发到 `server.listen` 地址。`self_url` 应填写客户端实际访问的 HTTPS 地址。

Pavonis 根据 `Host` 头将请求路由到站点，因此前置代理必须转发客户端原始的 `Host`。

前置代理还应传递客户端真实 IP，Pavonis 用它记录日志、做单客户端限速和 `ip_hash` 策略。下面每个示例都给出了反向代理配置和与之对应的 [`server`](../config/server) 配置。示例假设反向代理与 Pavonis 运行在同一台主机上。若 Pavonis 运行在 Docker 中，需监听 `0.0.0.0:8009`，并将反向代理连入的地址（如 Docker 网络的网关）填入 `trusted_proxy_ips`。

## Caddy

Caddy 会自动申请证书，且默认保留 `Host`，是最简单的选择。

```text
docker.example.com, gh.example.com {
    reverse_proxy 127.0.0.1:8009
}
```

Caddy 会将客户端 IP 追加到 `X-Forwarded-For`：

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'X-Forwarded-For'
```

## Nginx

Nginx 默认会将 `Host` 改写为上游地址，因此必须设置 `proxy_set_header Host`。证书按常规方式配置，此处省略。

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

TLS 由 Cloudflare 终结，`cloudflared` 默认保留 `Host`，源站无需开放端口。

```yaml
ingress:
  - hostname: docker.example.com
    service: http://127.0.0.1:8009
  - hostname: gh.example.com
    service: http://127.0.0.1:8009
  - service: http_status:404
```

Cloudflare 将客户端 IP 放在 `CF-Connecting-IP` 中。由于 Pavonis 只接受本机 `cloudflared` 的连接，信任 `127.0.0.1` 即可：

```yaml
server:
  listen: 127.0.0.1:8009
  trusted_proxy_ips:
    - '127.0.0.1/32'
  trusted_proxy_headers:
    - 'CF-Connecting-IP'
```
