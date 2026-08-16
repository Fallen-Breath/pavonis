---
title: 文档
order: 0
---

# Pavonis 文档

Pavonis 是一个用 Go 编写的多功能 HTTP 反向代理服务器，通过一份 YAML 配置文件即可同时运行多个不同类型的代理站点。

## 文档导航

### 📖 指引

了解 Pavonis 的基本概念，快速完成安装和配置。

- [快速上手](./guide/getting-started) — 安装、最小配置、启动运行
- [核心概念](./guide/concepts) — 站点、模式、路径前缀、IP 池等核心概念解释

### ✨ 功能特性

Pavonis 的核心能力介绍。

- [功能概览](./features/) — 各种 Sites 模式、限速、IP 池等功能全览

### 📦 站点模式 (Sites)

每种 `mode` 的详细用途、配置项与示例。

- [GitHub 文件加速](./sites/gh-proxy) — `gh_proxy` 模式
- [容器镜像代理（单仓库）](./sites/container-registry-single) — `container_registry_single` 模式
- [容器镜像代理（任意仓库）](./sites/container-registry-any) — `container_registry_any` 模式
- [通用 HTTP 反向代理](./sites/http-proxy) — `http` 模式
- [PyPI 镜像](./sites/pypi) — `pypi` 模式
- [HuggingFace 下载代理](./sites/hugging-face) — `hugging_face` 模式
- [测速](./sites/speed-test) — `speed_test` 模式

### ⚙️ 配置参考

所有配置项的详细说明。

- [配置概览](./config/) — 完整配置文件示例与顶层字段速览
- [ServerConfig (服务器设置)](./config/server)
- [RequestConfig (出站请求设置)](./config/request)
- [ResponseConfig (响应设置)](./config/response)
- [ResourceLimitConfig (资源限制)](./config/resource-limit)
- [SiteConfig (站点配置)](./config/sites)
- [ContainerRegistryAuthConfig (容器仓库认证)](./config/auth)
- [DiagnosticsConfig (诊断服务)](./config/diagnostics)
