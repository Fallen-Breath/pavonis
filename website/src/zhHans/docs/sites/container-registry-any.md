---
title: 容器镜像代理（任意仓库）
order: 3
---

# 容器镜像代理——任意仓库（container_registry_any）

## 简介

`container_registry_any` 模式可以代理任意 OCI 兼容的容器镜像仓库，适合搭建通用镜像加速器。客户端只需在镜像名中写出镜像源，不用为每个仓库单独配置站点。

**该模式只用于拉取上游的公开镜像**：不支持推送（只接受 `GET` 和 `HEAD` 请求），也不支持登录上游账号来拉取私有镜像。

## 使用方式

镜像名写成 `{Pavonis 地址}/{镜像源}/{镜像名}`。例如站点绑定在 `mirror.example.com` 时：

```bash
# Docker Hub 官方镜像
docker pull mirror.example.com/docker.io/library/nginx:latest

# GHCR 镜像
docker pull mirror.example.com/ghcr.io/user/image:tag

# NVCR 镜像
docker pull mirror.example.com/nvcr.io/nvidia/pytorch:latest
```

镜像源必须写完整的主机名，即包含 `.` 或 `:`，或者是 `localhost`。因此 Docker Hub 镜像要写全为 `docker.io/library/nginx`，不能像直接使用 docker 时那样省略成 `nginx`。`docker.io` 和 `registry-1.docker.io` 两种写法等价。

containerd 的透明镜像（在 `hosts.toml` 中配置 mirror）无法使用本模式，因为那种方式下镜像名里没有镜像源。另外，不支持 `_catalog` 接口。

## 配置项（settings）

> 对应 Go 配置类型：`ContainerRegistryAnySiteSettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `allow_list` | `bool` | `false` | 是否允许 `tags/list`、`referrers` 接口 |
| `repos_whitelist` | `[]string` | `[]` | 仓库白名单（包含 registry 主机名） |
| `repos_blacklist` | `[]string` | `[]` | 仓库黑名单（包含 registry 主机名） |
| `auth` | [`ClientAuthConfig`](../config/auth) | 无 | 客户端访问认证配置 |

### 白黑名单格式

格式为 `registry/命名空间/仓库名`，支持通配符 `*`。`docker.io` 和 `registry-1.docker.io` 视为同一个：

```yaml
repos_whitelist:
  - 'ghcr.io/my-org/*'
  - 'docker.io/library/*'
```

## 配置示例

### 基础用法

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
```

### 只允许特定 registry

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
    settings:
      repos_whitelist:
        - 'ghcr.io/*'
        - 'docker.io/library/*'
```

### 启用客户端认证

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

启用认证后，客户端需要先 `docker login mirror.example.com`。用户名和密码就是 Pavonis 账号。

详细的认证配置说明请参阅 [ClientAuthConfig](../config/auth)。
