---
title: HTTPS 与反向代理
order: 2
---

# HTTPS 与反向代理

对外提供 HTTPS 有两种方式：配置 `server.tls` 由 Pavonis 直接监听 HTTPS，或在 Pavonis 前面部署反向代理，由前置代理负责 HTTPS 再通过 HTTP 转发到 `server.listen`。无论采用哪种方式，站点的 `self_url` 都应填写客户端实际访问的 HTTPS 地址。

## 直接提供 HTTPS

已有证书文件且 Pavonis 可以直接占用端口时，配置 `server.tls` 即可对外提供 HTTPS，证书更新后会自动重新加载：

```yaml
server:
  listen: 0.0.0.0:443
  tls:
    certificates:
      - cert_file: /etc/pavonis/certs/fullchain.pem
        key_file: /etc/pavonis/certs/privkey.pem
```

字段说明与多证书匹配规则见 [`ServerConfig`](../config/server#servertlsconfig)。Pavonis 不会自动申请证书，也不会监听额外端口将 HTTP 重定向到 HTTPS。如果需要自动管理证书、与同机其他服务共用 `443` 端口，或不直接开放源站端口，请使用前置反向代理。

## 使用前置反向代理

Pavonis 根据 `Host` 头将请求路由到站点，因此前置代理必须转发客户端原始的 `Host`。

前置代理还应传递客户端真实 IP，Pavonis 用它记录日志、做 `per_client` 资源限制和 `ip_hash` 策略。下面每个示例都给出了反向代理配置和与之对应的 [`server`](../config/server) 配置。示例假设反向代理与 Pavonis 运行在同一台主机上。若 Pavonis 运行在 Docker 中，需监听 `0.0.0.0:8009`，并将反向代理连入的地址（如 Docker 网络的网关）填入 `trusted_proxy_ips`。

### Caddy

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

### Nginx

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

### Cloudflare Tunnel

HTTPS 由 Cloudflare 处理，`cloudflared` 默认保留 `Host`，源站无需开放端口。

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
