---
title: 容器镜像代理（任意仓库）
order: 3
---

# 容器镜像代理——任意仓库（container_registry_any）

## 简介

`container_registry_any` 模式通过**在请求路径中嵌入目标仓库的主机名**，实现对任意 OCI 兼容容器镜像仓库的透明代理。适合搭建通用镜像加速器，客户端通过修改镜像地址即可使用。

**该模式只支持拉取（pull），不支持推送（push）。在 HTTP 层面表现为只接受 `GET` 和 `HEAD` 请求。**

## 请求格式

请求路径格式为：

```
/{registry-host}/v2/{image-name}/{...}
```

例如，若 Pavonis 的 `container_registry_any` 站点绑定在 `mirror.example.com`：

```bash
# 拉取 Docker Hub 官方镜像
docker pull mirror.example.com/registry-1.docker.io/library/nginx:latest

# 拉取 GHCR 镜像
docker pull mirror.example.com/ghcr.io/user/image:tag

# 拉取 NVCR 镜像
docker pull mirror.example.com/nvcr.io/nvidia/pytorch:latest
```

## 配置项（settings）

> 对应 Go 配置类型：`ContainerRegistryAnyProxySettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `allow_list` | `bool` | `false` | 是否允许 `_catalog` 和 `tags/list` 接口 |
| `repos_whitelist` | `[]string` | `[]` | 仓库白名单（包含 registry 主机名） |
| `repos_blacklist` | `[]string` | `[]` | 仓库黑名单（包含 registry 主机名） |
| `auth` | [`ContainerRegistryAuthConfig`](../config/auth) | 无 | 客户端访问认证配置 |

### 白黑名单格式

格式为 `registry-host/命名空间/仓库名`，支持通配符 `*`：

```yaml
repos_whitelist:
  - 'ghcr.io/my-org/*'
  - 'registry-1.docker.io/library/*'
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
        - 'registry-1.docker.io/library/*'
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

详细的认证配置说明请参阅 [ContainerRegistryAuthConfig](../config/auth)。
