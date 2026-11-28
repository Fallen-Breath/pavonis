# Pavonis

<img src="website/src/public/logo.svg" align="right" width="192" alt="Pavonis logo">

[![CI](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml/badge.svg)](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Fallen-Breath/pavonis)](https://github.com/Fallen-Breath/pavonis/releases)
[![Docker Hub](https://img.shields.io/docker/v/fallenbreath/pavonis?label=docker&sort=semver)](https://hub.docker.com/r/fallenbreath/pavonis)
[![License](https://img.shields.io/github/license/Fallen-Breath/pavonis)](LICENSE)

**English** | [中文](README_zh.md)

A multi-purpose reverse proxy for container registries, GitHub assets, Hugging Face, PyPI and more.
Run all your proxy sites from one deployment and one config file

Documentation: https://pavonis.cc/

## Features

### Upstream Proxying

Each site uses a [site mode](https://pavonis.cc/docs/sites/) tailored to its upstream, following or rewriting redirects and download links so traffic stays on the proxy:

- Proxy a single OCI registry such as Docker Hub, GHCR or NVCR, or any public registry chosen in the image name
- Proxy GitHub releases, archives, raw files, gists and git clone, as well as Hugging Face model and dataset downloads
- Proxy PyPI for pip, uv and Poetry, and the npm registry for npm, pnpm and yarn
- Route path prefixes to different HTTP upstreams with response body rewriting for [Cargo, Conda, Go, Gradle, Maven and more](https://pavonis.cc/docs/guide/ecosystems), or proxy any URL given in the request path

### Outbound Connections

- Send upstream requests from an outbound IP pool (e.g. an IPv6 /48 subnet) to spread per-IP rate limits
- Route upstream traffic through HTTP or SOCKS5 proxies with automatic failover

### Traffic Control and Monitoring

- Authenticate clients and restrict which repositories, packages or domains can be proxied
- Cap bandwidth, request rate and concurrency per client or across sites so a few heavy clients can't exhaust the server
- Export per-site Prometheus metrics for traffic, latency and upstream responses, with an [example Grafana dashboard](grafana.json)

## Quick Start

Write a `config.yml`. This one proxies Docker Hub on `docker.example.com`:

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

Run it with Docker. Pavonis listens on port `8009` by default:

```bash
docker run -d --name pavonis -p 8009:8009 \
  -v ./config.yml:/etc/pavonis/config.yml \
  fallenbreath/pavonis:latest
```

Expose it over [HTTPS](https://pavonis.cc/docs/guide/https), then pull through the proxy:

```bash
docker pull docker.example.com/library/nginx
```

Pre-built binaries are available on [GitHub Releases](https://github.com/Fallen-Breath/pavonis/releases).
See [Getting Started](https://pavonis.cc/docs/guide/getting-started) for more, and the [Config Reference](https://pavonis.cc/docs/config/) for all options.

## Status

Pavonis is pre-1.0. Config options may still change between minor versions.

## License

Licensed under the [GNU Affero General Public License v3.0](LICENSE).
