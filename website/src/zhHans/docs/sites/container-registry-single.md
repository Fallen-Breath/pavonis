---
title: 容器镜像代理（单仓库）
order: 2
---

# 容器镜像代理——单仓库（container_registry_single）

## 简介

`container_registry_single` 模式将 Pavonis 作为**固定上游仓库**的 OCI/Docker 容器镜像代理。适合为某个具体的镜像仓库（如 Docker Hub、GHCR、NVCR 等）建立专用镜像站。

该模式完整支持 OCI Distribution Spec（v1/v2 API），可兼容 `docker pull/push`、`crictl`、`containerd` 等容器工具。

**`upstream_v2_url`、`upstream_v1_url`、`upstream_auth_realm_url` 都不填时，默认代理 Docker Hub。** 这三个 URL 描述的是同一个仓库：只要填了其中任意一个，其余都不会回退到 Docker Hub。代理其他仓库时填写 `upstream_v2_url`，按需再填另外两个。

## 配置项（settings）

> 对应 Go 配置类型：`ContainerRegistrySingleProxySettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `upstream_v2_url` | `string` | 见上方 | 上游仓库的 `/v2` 端点 URL（不含尾部 `/`）。填写了另外两个 URL 之一时必填 |
| `upstream_v1_url` | `string` | 见上方 | 上游仓库的 `/v1` 端点 URL（可选，不含尾部 `/`）|
| `upstream_auth_realm_url` | `string` | 自动发现 | 上游认证 realm URL |
| `allow_push` | `bool` | `false` | 是否允许 push 操作（`/v2` 下的 `PUT`/`POST`/`PATCH`/`DELETE` 请求）。不影响 `/auth` |
| `allow_list` | `bool` | `false` | 是否允许列举类操作：`_catalog`、`tags/list` 接口，以及 `/v1` API（`docker search` 使用） |
| `repos_whitelist` | `[]string` | `[]` | 仓库白名单 |
| `repos_blacklist` | `[]string` | `[]` | 仓库黑名单 |
| `auth` | [`ClientAuthConfig`](../config/auth) | 无 | 客户端访问认证配置 |

### 白黑名单格式

格式为 `命名空间/仓库名`，支持通配符 `*`，也可以只写 `命名空间`（匹配该命名空间下所有仓库）：

```yaml
repos_whitelist:
  - 'library/nginx'       # 匹配该仓库及其下级仓库
  - 'my-org/*'            # 匹配 my-org 下所有仓库
```

白名单和黑名单均按完整路径段进行前缀匹配。以 `library/nginx` 为例：

| 仓库名 | 是否匹配 |
|--------|----------|
| `library/nginx` | 是 |
| `library/nginx/extra` | 是 |
| `library/nginx/extra/deeper` | 是 |
| `library/nginx-extra` | 否 |
| `library/other` | 否 |

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

详细的认证配置说明请参阅 [ClientAuthConfig](../config/auth)。

### 用路径前缀做免登录的私有加速器

如果只想给 Docker 配镜像加速器，又不想在每台机器上 `docker login`，可以把一段随机字符串作为 `path_prefix`，让它充当访问密钥：

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    path_prefix: /Mys3cR3tPa55w0rd
    mode: container_registry_single
    self_url: https://docker.example.com
```

然后在 `/etc/docker/daemon.json` 中把带前缀的地址设为加速器：

```json
{
  "registry-mirrors": ["https://docker.example.com/Mys3cR3tPa55w0rd"]
}
```

之后 `docker pull nginx` 等 Docker Hub 镜像会自动经过该站点。不带前缀的请求不会匹配此站点（同一 `host` 下没有其他站点时返回 `404`）。

这种方式**只适用于 Docker 的 `registry-mirrors`**：仓库地址里带路径不是通用写法，`containerd`、`podman`、`crictl` 等其他工具大概率无法使用。另外，`registry-mirrors` 本身只对 Docker Hub 镜像生效。

前缀相当于一个所有人共用的密码：拿到 `daemon.json` 或访问日志的人都能使用，泄露后只能更换前缀。需要区分用户或撤销单个用户时，请改用[客户端认证](#启用客户端认证)。

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
