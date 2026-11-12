# Pavonis

<img src="website/src/public/logo.svg" align="right" width="192" alt="Pavonis logo">

[![CI](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml/badge.svg)](https://github.com/Fallen-Breath/pavonis/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Fallen-Breath/pavonis)](https://github.com/Fallen-Breath/pavonis/releases)
[![Docker Hub](https://img.shields.io/docker/v/fallenbreath/pavonis?label=docker&sort=semver)](https://hub.docker.com/r/fallenbreath/pavonis)
[![License](https://img.shields.io/github/license/Fallen-Breath/pavonis)](LICENSE)

A multi-purpose reverse proxy for container registries, GitHub assets, HuggingFace, PyPI and more

One binary and one YAML file run any number of proxy sites side by side.
Each site binds to a hostname (and optionally a path prefix) and picks a mode.
Rate limiting, the outbound IP pool and monitoring are shared by all sites,
so a single deployment can replace a set of separately maintained mirrors

Documentation: https://pavonis.cc/

## Site Modes

| Use case                                        | Mode                        |
|-------------------------------------------------|-----------------------------|
| Mirror one container registry (Docker Hub, GHCR, ...) | `container_registry_single` |
| Proxy any container registry, chosen by request path | `container_registry_any`    |
| GitHub releases, archives, raw files and gists  | `gh_proxy`                  |
| HuggingFace model and dataset downloads         | `hugging_face`              |
| PyPI index for pip and friends                  | `pypi`                      |
| General HTTP reverse proxy with path mappings   | `http`                      |
| Proxy any URL given in the request path         | `any_proxy`                 |

See [Site Modes](https://pavonis.cc/docs/sites/) for what each mode supports

## Highlights

- Outbound IP pool: send upstream requests from a whole subnet (e.g. an IPv6 /48), picked randomly or pinned per client, to spread upstream per-IP rate limits
- Upstream request proxies: route upstream traffic through HTTP / SOCKS proxies, with failover and optional direct fallback
- Resource limits: per-client and global bandwidth limits, per-client request rate limits, request timeout
- Access control: client authentication for registry sites with hot-reloaded user files, repository allow / deny lists, domain deny lists
- Monitoring: Prometheus metrics and pprof on a separate diagnostics port, with an [example Grafana dashboard](grafana.json)

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

Pre-built binaries are available on [GitHub Releases](https://github.com/Fallen-Breath/pavonis/releases).
See [Getting Started](https://pavonis.cc/docs/guide/getting-started) for more, and the [Config Reference](https://pavonis.cc/docs/config/) for all options

Pavonis is pre-1.0. Config options may still change between minor versions
