# Pavonis

**Still under development**

An HTTP reverse proxy supporting advanced scenarios like proxying container registry, 
GitHub asset downloading, and PyPI index

## Features

- Multiple reversed proxy modes:
    - General HTTP reversed proxy
        - Supports path mappings
        - Supports following redirect response
    - URL-based general HTTP proxy
    - [Container Registry](https://distribution.github.io/distribution/) proxy
        - Supports single-registry and multi-registry proxying
        - Supports pull, with optional push in single-registry mode
        - Supports customized authorization
    - [GitHub](https://github.com/) proxy, which behaves like [ghproxy](https://ghproxy.link/)
    - [PyPI](https://pypi.org/) index proxy
    - [HuggingFace](https://huggingface.co/) CLI download proxy
- Resource control
    - Request rate limit
    - Traffic rate limit
    - Request timeout
- IP Pooling
    - Send the downstream utilizing a full IP subnet

## Website

https://pavonis.cc/

## Usage

https://hub.docker.com/r/fallenbreath/pavonis

## Config

[Configuration reference](https://pavonis.cc/docs/config/)

## Monitoring

[Example Grafana dashboard](grafana.json)

## TODO

maintainability

- [x] self inspect site mode
- [x] prometheus

config

- [x] TrustedProxyIps, TrustedProxyHeaders
- [x] container registry whitelist
- [x] path prefix
- [x] max redirect (follow redirect config)
- [ ] burst traffic limit pool
- [ ] global traffic limit
- [ ] better config dump

qol

- [x] logging prefix

project

- [x] doc
- [x] website
