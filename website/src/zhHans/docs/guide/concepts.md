---
title: 核心概念
order: 2
---

# 核心概念

本页介绍 Pavonis 配置文件中的基础概念，帮助你在部署时理解各字段的含义。读完本页，你应该能够看懂并写出一份基本可用的配置文件。

## 站点（Site）

**站点（site）**是 Pavonis 的基本调度单元。每个站点绑定一个或多个主机名（`host`）、选择工作模式（`mode`），并携带该模式所需的具体设置（`settings`）。Pavonis 根据请求的 `Host` 头（以及路径前缀）将请求路由到对应站点处理。

```yaml
sites:
  - id: dockerhub            # 站点唯一标识，用于日志区分（可选）
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      upstream_v2_url: https://registry-1.docker.io
```

一个站点可以绑定**多个主机名**，或将 `host` 设为 `"*"` 作为兜底处理器，匹配所有未被其他站点命中的请求：

```yaml
sites:
  - host:
      - proxy.example.com
      - proxy2.example.com
    mode: gh_proxy

  - host: '*'
    mode: http
    settings:
      mappings:
        - prefix: /
          destination: https://upstream.example.com
```

## 模式（Mode）

`mode` 字段决定站点的代理行为，目前支持以下模式：

| 模式值 | 说明 |
|--------|------|
| `gh_proxy` | GitHub Releases / Raw 文件 / Gist 加速 |
| `container_registry_single` | 代理指定容器镜像仓库（如 Docker Hub、GHCR） |
| `container_registry_any` | 通用容器镜像加速，按路径中的主机名路由 |
| `http` | 通用 HTTP 反向代理，支持多路径映射 |
| `pypi` | PyPI 包索引镜像，自动重写下载链接 |
| `hugging_face` | HuggingFace 模型/数据集下载代理 |
| `speed_test` | 上传/下载测速端点 |

每种模式的详细参数说明见[站点模式](../sites/)。

## 路径前缀（path_prefix）

`path_prefix` 允许同一域名下的不同路径路由到不同站点，实现"一个域名，多种代理服务"：

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi

  - host: proxy.example.com
    mode: speed_test
    path_prefix: /speedtest
```

- 必须以 `/` 开头
- 路径前缀越长，优先级越高（最长前缀匹配）

## self_url

`self_url` 是本站点对外可访问的完整 URL，格式为 `scheme://host`（不含路径，不以 `/` 结尾）。

部分模式需要用它重写响应中的回调地址（如容器仓库的认证 realm、HuggingFace 的重定向链接），确保客户端的后续请求仍然经过 Pavonis。

| 模式 | 是否需要 `self_url` |
|------|---------------------|
| `container_registry_single` | ✅ 必填 |
| `container_registry_any` | ✅ 必填 |
| `hugging_face` | ✅ 必填 |
| `gh_proxy`（开启 `raw_text_url_rewrite` 时） | ✅ 需要 |

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com   # 只含 scheme + host，无尾部斜杠
```

## 下一步

掌握基础概念后，可以继续探索：

- [功能特性](../features/) — IP 池、限速、认证等跨站点共享能力
- [站点模式](../sites/) — 每种模式的详细参数与示例
- [配置参考](../config/) — 完整配置结构体文档
