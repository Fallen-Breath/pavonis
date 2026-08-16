---
title: ContainerRegistryAuthConfig (容器仓库认证)
order: 6
---

# ContainerRegistryAuthConfig (容器仓库认证)

`auth` 是 `container_registry_single` 和 `container_registry_any` 两种模式下 `settings` 中的认证配置块。启用后，客户端需要提供有效凭据才能通过 Pavonis 访问容器镜像仓库。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | `bool` | `false` | 是否启用客户端认证 |
| `users` | `[]User` | `[]` | 内联用户列表，每项含 `name` 和 `password` |
| `users_file` | `string` | 无 | 外部用户文件路径（YAML 格式），与 `users` 合并生效 |
| `users_file_reload_interval` | `duration` | 无 | 外部用户文件的定期热重载间隔，不填则只在启动时加载一次。最小值须大于 `1s` |

### 用户名/密码限制

- `name` 和 `password` 均不能为空
- `name` 和 `password` 均不能包含字符 `$` 或 `:`

## 内联用户（users）

直接在配置文件中定义用户列表：

```yaml
settings:
  auth:
    enabled: true
    users:
      - name: alice
        password: alice-secret
      - name: bob
        password: bob-secret
```

## 外部用户文件（users_file）

将用户列表存放在独立的 YAML 文件中，便于与配置文件分离管理：

**`users.yml` 格式：**

```yaml
users:
  - name: alice
    password: alice-secret
  - name: bob
    password: bob-secret
```

**在配置文件中引用：**

```yaml
settings:
  auth:
    enabled: true
    users_file: /etc/pavonis/users.yml
```

### 热重载

配置 `users_file_reload_interval` 后，Pavonis 会在后台定期重新读取用户文件，无需重启即可生效新增或删除的用户：

```yaml
settings:
  auth:
    enabled: true
    users_file: /etc/pavonis/users.yml
    users_file_reload_interval: 5m    # 每 5 分钟重载一次
```

如果同时配置了 `users` 和 `users_file`，两份列表会被合并，所有用户均有效。

## 认证机制详解

启用 auth 后，Pavonis 会将 OCI 标准的认证流程中的 `realm` URL 重写为指向自身的 `/auth` 端点。客户端（如 `docker login`）会按照以下流程进行认证：

1. 客户端请求 `/v2/` → 收到 `401` 响应，`WWW-Authenticate` 头部中含有 `realm=https://your-pavonis/auth`
2. 客户端携带凭据请求 Pavonis 的 `/auth` 端点
3. Pavonis 验证凭据，若合法则向上游发起真实的 token 请求，将 token 返回给客户端
4. 客户端使用 token 继续请求镜像数据

### 传递上游凭据（高级用法）

当上游仓库本身也需要认证（例如私有 GHCR 仓库）时，客户端可以在凭据中**同时传递 Pavonis 凭据和上游凭据**，用 `$` 符号分隔：

```
用户名格式：pavonis用户名$上游用户名
密码格式：  pavonis密码$上游密码
```

例如，使用 Docker CLI 登录：

```bash
docker login docker.example.com \
  -u "alice\$upstream-user" \
  -p "alice-secret\$upstream-token"
```

如果只提供 Pavonis 凭据（不带 `$`），Pavonis 会以匿名方式向上游发起请求（适用于上游为公开仓库的场景）。

## 完整配置示例

### 单仓库代理，启用认证 + 热重载

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      auth:
        enabled: true
        users:
          - name: admin
            password: admin-password
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 10m
```

### 任意仓库代理，启用认证

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
    settings:
      auth:
        enabled: true
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 5m
```
