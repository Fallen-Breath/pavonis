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
    - [HuggingFace](https://huggingface.co/) CLI download proxy
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

- [ ] doc
- [ ] website
