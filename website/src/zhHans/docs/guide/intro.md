---
title: 简介
---

Pavonis（孔雀座），一个通用的、开箱即用 HTTP 反向代理，可用于反代互联网上各类数据资源站点

支持如下几种反代模式：

- 通用 HTTP 反代：HTTP 将请求转发至任意下游服务上，可反代各类 API 服务、Maven 仓库等等
- 容器镜像仓库反代：反代任意容器镜像，如 Docker Hub、ghcr 等；支持 pull、push，支持黑白名单、自定义鉴权
- GH-Proxy 反代：方便快捷的 [GitHub](https://github.com/) 资源下载，就如 [ghproxy](https://ghproxy.link/) 一样
- [PyPI](https://pypi.org/) 索引反代：加速 `pip install` 等命令的 Python 包安装过程
- [HuggingFace](https://huggingface.co/) 下载反代：完整接管全下载链路的一个 `HF_ENDPOINT`

支持如下高级反代功能：

- 跟随 / 改写 HTTP 重定向的响应回报
- 改写请求包中的 HTTP header
- 请求频率限制、带宽限制
- 使用给定 IP 池中的 IP 来发起请求

应用场景：

- 网络受限环境下的请求加速
- 内网环境下的请求出口枢纽
- 使用 IP 池绕过目标站点频控
