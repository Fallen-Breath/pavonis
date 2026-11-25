---
title: PyPI Mirror
order: 6
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

> Corresponding Go config type: `PypiSiteSettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `upstream_simple_url` | `string` | `https://pypi.org/simple` | Upstream Simple API URL (no trailing `/`) |
| `upstream_files_url` | `string` | `https://files.pythonhosted.org` | Upstream file download URL (no trailing `/`) |
| `packages_whitelist` | `[]string` | `[]` | Package allowlist. Requests for packages not on the list return `403` |
| `packages_blacklist` | `[]string` | `[]` | Package denylist. Requests for packages on the list return `403` |

::: tip
`upstream_simple_url` and `upstream_files_url` must either both be set or both be left unset.
:::

### Allow/Deny List Format

Each entry is a PyPI package name, without wildcards. Matching normalizes names according to [PEP 503](https://peps.python.org/pep-0503/#normalized-names) and compares them for exact equality, so case is ignored and runs of `-`, `_`, and `.` are treated as equivalent.

When both `packages_whitelist` and `packages_blacklist` are configured, a package must be on the allowlist and not on the denylist for the request to pass.

When either list is configured, the `/simple/` root index is still allowed. Under `/files/...`, only standard wheel (`.whl`) and source distribution (`.tar.gz`) archives and their `.metadata` files can have their package names identified, so requests for other files return `403`.

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

### Restrict Allowed Packages

```yaml
sites:
  - id: pypi
    host: pypi.example.com
    mode: pypi
    settings:
      packages_whitelist:
        - requests
        - urllib3
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

Or in `pyproject.toml`:

```toml
[[tool.uv.index]]
url = "https://pypi.example.com/simple/"
default = true
```

### Poetry

```toml
# pyproject.toml
[[tool.poetry.source]]
name = "pavonis"
url = "https://pypi.example.com/simple/"
priority = "primary"
```
