# Pavonis

**Still under early development**

An HTTP reverse proxy supporting advanced scenarios like proxying container registry, 
GitHub asset downloading, and PyPI index

## Features

- Multiple reversed proxy modes:
    - General HTTP reversed proxy
        - Supports path mappings
        - Supports following redirect response
    - [Container Registry](https://distribution.github.io/distribution/) proxy
        - Supports both pull and push
        - Supports customized authorization
    - [GitHub](https://github.com/) proxy, which behaves like [ghproxy](https://ghproxy.link/)
    - [PyPI](https://pypi.org/) index proxy
- Resource control
    - Request rate limit
    - Traffic rate limit
    - Request timeout
- IP Pooling
    - Send the downstream utilizing a full IP subnet

## Demo

https://pavonis.cc/

## Usage

https://hub.docker.com/r/fallenbreath/pavonis

## Config

TODO

## TODO

maintainability

- [x] self inspect site mode
- [ ] prometheus

config

- [ ] lru cache size
- [x] TrustedProxyIps, TrustedProxyHeaders
- [x] container registry whitelist
- [x] path prefix
- [x] max redirect (follow redirect config)
- [ ] configurable gh proxy
- [ ] burst traffic limit pool
- [ ] global traffic limit

qol

- [x] logging prefix

project

- [ ] doc
- [ ] website
