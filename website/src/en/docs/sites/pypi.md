---
title: PyPI Mirror
order: 5
---

# PyPI Mirror (pypi)

## Overview

The `pypi` mode turns Pavonis into a PyPI index mirror, automatically rewriting download links in responses so that all requests from `pip`, `poetry`, `uv` and similar tools — both index queries and file downloads — pass through Pavonis.

## Routing Rules

| Request Path | Forwarded To |
|-------------|-------------|
| `/simple/...` | `upstream_simple_url` (package index API) |
| `/files/...` | `upstream_files_url` (package file downloads) |

Pavonis automatically rewrites download URLs in upstream Simple API responses (both HTML and JSON) to point to this site's `/files/...` path. Clients never need to know the actual upstream file server address.

## Settings

> Corresponding Go config type: `PypiRegistrySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `upstream_simple_url` | `string` | `https://pypi.org/simple` | Upstream Simple API URL (no trailing `/`) |
| `upstream_files_url` | `string` | `https://files.pythonhosted.org` | Upstream file download URL (no trailing `/`) |

::: tip
`upstream_simple_url` and `upstream_files_url` must either both be set or both be left unset.
:::

## Configuration Examples

### Proxy the Official PyPI (default)

```yaml
sites:
  - id: pypi
    host: pypi.example.com
    mode: pypi
```

### Proxy Another PyPI Mirror

Using Tsinghua University mirror as an example:

```yaml
sites:
  - id: pypi
    host: pypi.example.com
    mode: pypi
    settings:
      upstream_simple_url: https://pypi.tuna.tsinghua.edu.cn/simple
      upstream_files_url: https://pypi.tuna.tsinghua.edu.cn/packages
```

### With path_prefix

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi
```

## Client Usage

### pip

```bash
pip install requests -i https://pypi.example.com/simple/
```

Or in `pip.conf`:

```ini
[global]
index-url = https://pypi.example.com/simple/
```

### uv

```bash
uv pip install requests --index-url https://pypi.example.com/simple/
```

### pyproject.toml (uv / Poetry)

```toml
[[tool.uv.index]]
url = "https://pypi.example.com/simple/"
default = true
```
