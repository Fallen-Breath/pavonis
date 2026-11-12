# Pavonis

<img src="website/src/public/logo.svg" align="right" width="192" alt="Pavonis logo">

[![CI](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml/badge.svg)](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Fallen-Breath/pavonis)](https://github.com/Fallen-Breath/pavonis/releases)
[![Docker Hub](https://img.shields.io/docker/v/fallenbreath/pavonis?label=docker&sort=semver)](https://hub.docker.com/r/fallenbreath/pavonis)
[![License](https://img.shields.io/github/license/Fallen-Breath/pavonis)](LICENSE)

A multi-purpose reverse proxy for container registries, GitHub assets, HuggingFace, PyPI and more.
Run all your mirrors from one deployment and one config file

Documentation: https://pavonis.cc/

## Site Modes

| Use case                                              | Mode                        |
|-------------------------------------------------------|-----------------------------|
| Mirror one container registry (Docker Hub, GHCR, ...) | `container_registry_single` |
| Proxy any container registry, chosen by request path  | `container_registry_any`    |
| GitHub releases, archives, raw files and gists        | `gh_proxy`                  |
| HuggingFace model and dataset downloads               | `hugging_face`              |
| PyPI index for pip and friends                        | `pypi`                      |
| General HTTP reverse proxy with path mappings         | `http`                      |
| Proxy any URL given in the request path               | `any_proxy`                 |

See [Site Modes](https://pavonis.cc/docs/sites/) for what each mode supports

## Highlights

- Outbound IP pool: send upstream requests from a whole subnet (e.g. an IPv6 /48) to spread upstream per-IP rate limits
- Upstream proxies: route upstream traffic through HTTP / SOCKS proxies
- Access control: authenticate registry clients and restrict which repositories or domains can be proxied
- Rate limiting and monitoring: cap bandwidth and request rates per client, and export Prometheus metrics with an [example Grafana dashboard](grafana.json)

## Quick Start

Write a `config.yml`. This one turns `docker.example.com` into a Docker Hub mirror:

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

Pavonis serves plain HTTP, so put a TLS-terminating reverse proxy in front of it. Then pull through the mirror:

```bash
docker pull docker.example.com/library/nginx
```

Pre-built binaries are available on [GitHub Releases](https://github.com/Fallen-Breath/pavonis/releases).
See [Getting Started](https://pavonis.cc/docs/guide/getting-started) for more, and the [Config Reference](https://pavonis.cc/docs/config/) for all options

Pavonis is pre-1.0. Config options may still change between minor versions
