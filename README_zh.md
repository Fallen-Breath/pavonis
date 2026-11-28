# Pavonis

<img src="website/src/public/logo.svg" align="right" width="192" alt="Pavonis logo">

[![CI](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml/badge.svg)](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Fallen-Breath/pavonis)](https://github.com/Fallen-Breath/pavonis/releases)
[![Docker Hub](https://img.shields.io/docker/v/fallenbreath/pavonis?label=docker&sort=semver)](https://hub.docker.com/r/fallenbreath/pavonis)
[![License](https://img.shields.io/github/license/Fallen-Breath/pavonis)](LICENSE)

[English](README.md) | **中文**

面向容器镜像仓库、GitHub 资源、Hugging Face、PyPI 等场景的多功能反向代理。
只需一次部署与一份配置文件，即可集中运行你的所有代理站点

文档站点：https://pavonis.cc/zhHans/

## 功能特性

### 上游代理

每个站点使用针对其上游定制的[站点模式](https://pavonis.cc/zhHans/docs/sites/)，自动跟随或重写重定向与下载链接，使流量始终经过代理：

- 代理 Docker Hub、GHCR、NVCR 等单个 OCI 容器镜像仓库，或在镜像名中指定源仓库来代理任意公开仓库
- 代理 GitHub Release、源码归档、Raw 文件、Gist 与 git clone，以及 Hugging Face 模型和数据集下载
- 为 pip、uv、Poetry 代理 PyPI，并为 npm、pnpm、yarn 代理 npm registry
- 按路径前缀将请求路由到不同 HTTP 上游并改写响应体，支持代理 [Cargo、Conda、Go、Gradle、Maven 等生态](https://pavonis.cc/zhHans/docs/guide/ecosystems)，或直接代理请求路径中给出的任意 URL

### 出站连接

- 从出站 IP 池（如 IPv6 /48 子网）发起上游请求，分摊单 IP 速率限制
- 通过 HTTP 或 SOCKS5 代理转发上游流量，支持自动故障切换

### 流量控制与监控

- 对客户端进行身份认证，并限制允许代理的仓库、包或域名
- 按单客户端或跨站点限制带宽、请求频率和并发请求数，防止少数重度客户端耗尽服务器资源
- 按站点导出流量、延迟和上游响应的 Prometheus 指标，附带 [Grafana 仪表盘示例](grafana.json)

## 快速上手

编写 `config.yml`。以下配置在 `docker.example.com` 上代理 Docker Hub：

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

使用 Docker 启动。Pavonis 默认监听 `8009` 端口：

```bash
docker run -d --name pavonis -p 8009:8009 \
  -v ./config.yml:/etc/pavonis/config.yml \
  fallenbreath/pavonis:latest
```

配置 [HTTPS](https://pavonis.cc/zhHans/docs/guide/https) 对外提供访问后，即可通过该代理拉取镜像：

```bash
docker pull docker.example.com/library/nginx
```

预编译二进制可在 [GitHub Releases](https://github.com/Fallen-Breath/pavonis/releases) 下载。
更多说明请参阅[快速上手](https://pavonis.cc/zhHans/docs/guide/getting-started)，全部配置项请参阅[配置参考](https://pavonis.cc/zhHans/docs/config/)。

## 项目状态

Pavonis 目前处于 1.0 之前的阶段，次版本更新之间仍可能调整配置项。

## 开源许可

采用 [GNU Affero General Public License v3.0](LICENSE) 许可证开源。
