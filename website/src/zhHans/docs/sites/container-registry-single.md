---
title: 容器镜像代理（单仓库）
order: 2
---

# 容器镜像代理——单仓库（container_registry_single）

## 简介

`container_registry_single` 模式将 Pavonis 作为**固定上游仓库**的 OCI/Docker 容器镜像代理。适合为某个具体的镜像仓库（如 Docker Hub、GHCR、NVCR 等）建立专用镜像站。

该模式完整支持 OCI Distribution Spec（v1/v2 API），可兼容 `docker pull/push`、`crictl`、`containerd` 等容器工具。

**不填写任何 `settings` 时，默认代理 Docker Hub。**

## 配置项（settings）

> 对应 Go 配置类型：`ContainerRegistrySingleProxySettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `upstream_v2_url` | `string` | Docker Hub | 上游仓库的 `/v2` 端点 URL（不含尾部 `/`）|
| `upstream_v1_url` | `string` | Docker Hub | 上游仓库的 `/v1` 端点 URL（可选，不含尾部 `/`）|
| `upstream_auth_realm_url` | `string` | 自动发现 | 上游认证 realm URL；若不填，Pavonis 会从上游 `401` 响应中自动获取 |
| `allow_push` | `bool` | `false` | 是否允许 push 操作（`PUT`/`POST`/`PATCH`/`DELETE` 请求）|
| `allow_list` | `bool` | `false` | 是否允许 `_catalog` 和 `tags/list` 接口 |
| `repos_whitelist` | `[]string` | `[]` | 仓库白名单 |
| `repos_blacklist` | `[]string` | `[]` | 仓库黑名单 |
| `auth` | [`ContainerRegistryAuthConfig`](../config/auth) | 无 | 客户端访问认证配置 |

### 白黑名单格式

格式为 `命名空间/仓库名`，支持通配符 `*`，也可以只写 `命名空间`（匹配该命名空间下所有仓库）：

```yaml
repos_whitelist:
  - 'library/nginx'       # 精确匹配
  - 'my-org/*'            # 匹配 my-org 下所有仓库
```

## 配置示例

### 代理 Docker Hub（默认）

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

使用方式：

```bash
docker pull docker.example.com/library/nginx:latest
```

也可以通过设置镜像加速器使用（`/etc/docker/daemon.json`）：

```json
{
  "registry-mirrors": ["https://docker.example.com"]
}
```

### 代理 GHCR

```yaml
sites:
  - id: ghcr
    host: ghcr.example.com
    mode: container_registry_single
    self_url: https://ghcr.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token
```

### 代理 NVCR（NVIDIA）

```yaml
sites:
  - id: nvcr
    host: nvcr.example.com
    mode: container_registry_single
    self_url: https://nvcr.example.com
    settings:
      upstream_v2_url: https://nvcr.io/v2
      upstream_auth_realm_url: https://nvcr.io/proxy_auth
```

### 代理自托管仓库（如 Codeberg）

```yaml
sites:
  - id: codeberg
    host: codeberg.example.com
    mode: container_registry_single
    self_url: https://codeberg.example.com
    settings:
      upstream_v2_url: https://codeberg.org/v2
```

### 启用客户端认证

开启后，客户端需要提供用户名和密码才能使用此代理：

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
          - name: alice
            password: my-secret-password
```

详细的认证配置说明请参阅 [ContainerRegistryAuthConfig](../config/auth)。

### 允许仅特定镜像

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      repos_whitelist:
        - 'library/*'     # 只允许官方镜像
        - 'my-org/*'
```
