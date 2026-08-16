---
title: 快速上手
order: 1
---

# 快速上手

## 安装

### 使用预编译二进制

从 [GitHub Releases](https://github.com/Fallen-Breath/pavonis/releases) 下载对应平台的二进制文件，赋予执行权限后直接运行：

```bash
chmod +x pavonis
./pavonis --config config.yml
```

### 使用 Docker

```bash
docker run -d \
  --name pavonis \
  -p 8009:8009 \
  -v /path/to/config.yml:/app/config.yml \
  ghcr.io/fallen-breath/pavonis:latest
```

### 使用 Docker Compose

```yaml
services:
  pavonis:
    image: ghcr.io/fallen-breath/pavonis:latest
    restart: unless-stopped
    ports:
      - "8009:8009"
    volumes:
      - ./config.yml:/app/config.yml
```

## 最小配置示例

下面是一份最小可运行的配置文件，代理 Docker Hub 镜像：

```yaml
server:
  listen: 0.0.0.0:8009

sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

将上述内容保存为 `config.yml`，启动后即可将 `docker.example.com` 用作 Docker Hub 的镜像代理。

## 一份更完整的示例

下面的配置同时运行了多个代理站点：

```yaml
debug: false

server:
  listen: 0.0.0.0:8009

resource_limit:
  traffic_avg_mibps: 100
  traffic_burst_mib: 50

sites:
  # GitHub 文件加速
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy

  # Docker Hub 镜像代理
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com

  # GitHub Container Registry 代理
  - id: ghcr
    host: ghcr.example.com
    mode: container_registry_single
    self_url: https://ghcr.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token

  # PyPI 镜像
  - id: pypi
    host: pypi.example.com
    mode: pypi

  # HuggingFace 下载代理
  - id: huggingface
    host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
```

## 启动

```bash
./pavonis --config config.yml
```

启动后，服务器默认监听 `:8009`（若配置中未指定 `server.listen`）。
