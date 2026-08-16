---
title: Getting Started
order: 1
---

# Getting Started

## Installation

### Pre-built Binary

Download the binary for your platform from [GitHub Releases](https://github.com/Fallen-Breath/pavonis/releases), make it executable, and run:

```bash
chmod +x pavonis
./pavonis --config config.yml
```

### Docker

```bash
docker run -d \
  --name pavonis \
  -p 8009:8009 \
  -v /path/to/config.yml:/app/config.yml \
  ghcr.io/fallen-breath/pavonis:latest
```

### Docker Compose

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

## Minimal Config Example

The following is the smallest working config file, proxying Docker Hub:

```yaml
server:
  listen: 0.0.0.0:8009

sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
```

Save this as `config.yml`. Once started, `docker.example.com` will act as a Docker Hub mirror.

## A More Complete Example

The following config runs multiple proxy sites simultaneously:

```yaml
debug: false

server:
  listen: 0.0.0.0:8009

resource_limit:
  traffic_avg_mibps: 100
  traffic_burst_mib: 50

sites:
  # GitHub asset acceleration
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy

  # Docker Hub mirror
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com

  # GitHub Container Registry proxy
  - id: ghcr
    host: ghcr.example.com
    mode: container_registry_single
    self_url: https://ghcr.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token

  # PyPI mirror
  - id: pypi
    host: pypi.example.com
    mode: pypi

  # HuggingFace download proxy
  - id: huggingface
    host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
```

## Starting Pavonis

```bash
./pavonis --config config.yml
```

After startup, the server listens on `:8009` by default (if `server.listen` is not specified in the config).
