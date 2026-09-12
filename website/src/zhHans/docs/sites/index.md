---
title: 站点模式 (Sites)
order: 2
---

# 站点模式 (Sites)

每个站点（site）透过 `mode` 字段指定工作模式。本章节逐一介绍每种模式的用途、配置项与使用示例。

| 模式 | 配置字符串 | 用途 |
|------|-----------|------|
| [任意上游代理](./any-proxy) | `any_proxy` | 代理请求路径中指定的任意上游 URL |
| [GitHub 文件加速](./gh-proxy) | `gh_proxy` | 代理 GitHub Releases、Raw、Gist 等文件下载 |
| [容器镜像代理（单仓库）](./container-registry-single) | `container_registry_single` | 代理指定的 OCI 容器镜像仓库 |
| [容器镜像代理（任意仓库）](./container-registry-any) | `container_registry_any` | 通过路径中嵌入目标主机来代理任意仓库 |
| [通用 HTTP 反向代理](./http-proxy) | `http` | 按路径映射将请求转发到不同上游 |
| [PyPI 镜像](./pypi) | `pypi` | 代理 PyPI 包索引，重写下载链接 |
| [HuggingFace 下载代理](./hugging-face) | `hugging_face` | 加速 HuggingFace 模型与数据集下载 |
| [测速](./speed-test) | `speed_test` | 提供上传/下载测速端点 |
