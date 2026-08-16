---
title: 认证、请求头与出站代理
order: 3
---

# 认证、请求头与出站代理

## 容器仓库访问认证

`container_registry_single` 和 `container_registry_any` 模式支持启用**客户端认证**，要求访问者提供用户名和密码（HTTP Basic Auth）。

### 配置示例

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      auth:
        enabled: true
        users:
          - name: alice
            password: secret123
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 60s
```

`users`（内联）和 `users_file`（外部文件）可以同时使用，两者中的用户均有效。

### 用户文件热重载

`users_file` 指定一个外部 YAML 文件，格式与内联 `users` 列表相同：

```yaml
# users.yml
users:
  - name: alice
    password: secret123
  - name: bob
    password: password456
```

Pavonis 会按 `users_file_reload_interval` 定期重新加载此文件，**无需重启**即可更新用户列表。

### 透传上游凭据

若需要在认证通过后同时向上游仓库传递凭据，可以在用户名中用 `$` 分隔 Pavonis 用户名和上游用户名：

```
pavonis_user$upstream_user
```

客户端以 `pavonis_user` 登录 Pavonis，Pavonis 向上游转发时使用 `upstream_user` 的身份。

详见 [`ContainerRegistryAuthConfig`](../config/auth)。

## 请求头管理

### 自动清理

Pavonis 在向上游转发请求前，会自动删除可能暴露代理链信息的头部：

- **反代头部**：`Via`、`X-Forwarded-For` 等
- **Cloudflare 注入头部**：`CF-Connecting-IP`、`CF-Ray`、`CF-Visitor` 等所有 `CF-*` 开头的头部

这可以防止上游通过这些头部感知到 Pavonis 的存在。

### 自定义修改

除自动清理外，可以显式配置**添加、修改或删除**任意请求头与响应头：

```yaml
request:
  header:
    modify:
      X-Custom-Token: 'my-token'      # 添加或修改请求头
    delete:
      - 'X-Internal-Header'           # 删除指定请求头

response:
  header:
    modify:
      Access-Control-Allow-Origin: '*'
    delete:
      - 'X-Powered-By'
```

详见 [`RequestConfig`](../config/request) 和 [`ResponseConfig`](../config/response)。

## 出站代理

可以为所有上游请求统一指定一个 HTTP 出站代理，支持 `http://` 和 `socks5://` 协议：

```yaml
request:
  proxy: http://127.0.0.1:7890
```

所有经 Pavonis 转发的上游请求都会走此代理，适合在有网络访问限制的环境中部署。

详见 [`RequestConfig`](../config/request)。
