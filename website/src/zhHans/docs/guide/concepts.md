---
title: 核心概念
order: 3
---

# 核心概念

本页介绍 Pavonis 配置文件中的基础概念，帮助你在部署时理解各字段的含义。读完本页，你应该能够看懂并写出一份基本可用的配置文件。

## 上游与下游

本文档沿用反向代理的通行约定（与 Nginx、Caddy、Envoy 一致）：

- **下游（downstream）**：向 Pavonis 发起请求的客户端，如 `docker`、`pip`、浏览器
- **上游（upstream）**：Pavonis 将请求转发到的目标服务器，如 Docker Hub、GitHub、PyPI

```
客户端（下游）──▶ Pavonis ──▶ 目标服务器（上游）
```

配置项 `upstream_v2_url` 等以及"上游请求"之类的说法均按此约定。位于客户端与 Pavonis 之间的代理（Nginx、CDN）称为**前置代理**。

## 站点（Site）

**站点（site）**是 Pavonis 的基本调度单元。每个站点绑定一个或多个主机名（`host`）、选择工作模式（`mode`），并携带该模式所需的具体设置（`settings`）。Pavonis 根据请求的 `Host` 头（以及路径前缀）将请求路由到对应站点处理。

```yaml
sites:
  - id: dockerhub            # 站点唯一标识，用于日志区分（可选）
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      upstream_v2_url: https://registry-1.docker.io/v2
```

一个站点可以绑定**多个主机名**，或将 `host` 设为 `"*"` 作为兜底处理器，处理 `Host` 未匹配任何其他站点的请求：

```yaml
sites:
  - host:
      - proxy.example.com
      - proxy2.example.com
    mode: gh_proxy

  - host: '*'
    mode: http
    settings:
      destination: https://upstream.example.com
```

## 模式（Mode）

`mode` 字段决定站点的代理行为。可选的模式及各自的参数见[站点模式](../sites/)。

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

写法与匹配规则见 [`SiteConfig`](../config/sites)。

## self_url

`self_url` 是本站点对外可访问的完整 URL，格式为 `scheme://host`（不含路径，不以 `/` 结尾）。

部分模式需要用它重写响应中的回调地址（如容器仓库的认证 realm、HuggingFace 的重定向链接），确保客户端的后续请求仍然经过 Pavonis。

哪些情况必须配置 `self_url`，见 [`SiteConfig`](../config/sites#self-url-必填说明)。

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
