---
title: npm Mirror
order: 7
---

# npm Mirror (npm)

## Overview

The `npm` mode turns Pavonis into a read-only npm registry mirror, automatically rewriting download URLs in package metadata so that metadata queries and tarball downloads from `npm`, `pnpm`, `yarn` and similar tools all pass through Pavonis.

Pavonis only rewrites download URLs starting with `upstream_registry_url`. Other URLs are left unchanged.

Write operations such as publishing and login are not supported.

::: warning self_url Required
The `npm` mode requires `self_url` to be set, in the format `https://your-domain` (no path, no trailing `/`).
:::

## Supported Path Patterns

Pavonis allows the read-only endpoints needed for installation and verification by default. All other paths return `404`:

| Path Format | Description |
|-------------|-------------|
| `/{package}` | Package metadata |
| `/{package}/{version-or-tag}` | Metadata for a specific version or tag |
| `/{package}/-/{file}.tgz` | Tarball download |
| `/-/package/{package}/dist-tags` | Package dist-tags |
| `/-/npm/v1/attestations/{package}@{version}` | Package provenance attestations |
| `/-/npm/v1/keys` | Signing public keys |
| `/-/ping` | Health check |

The following endpoints are controlled by config flags and return `403` when disabled:

| Path | Config Field | Description |
|------|--------------|-------------|
| `/-/v1/search` | `allow_search` | Package search |
| `/-/npm/v1/security/advisories/bulk` | `allow_audit` | Security audit (`npm audit`) |

## Settings

> Corresponding Go config type: `NpmSiteSettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `upstream_registry_url` | `string` | `https://registry.npmjs.org` | Upstream npm registry URL (no trailing `/`) |
| `allow_search` | `bool` | `false` | Whether to allow the package search endpoint (`/-/v1/search`) |
| `allow_audit` | `bool` | `true` | Whether to allow the security audit endpoint (`/-/npm/v1/security/advisories/bulk`) |

## Configuration Examples

### Proxy the Official npm Registry (default)

```yaml
sites:
  - id: npm
    host: npm.example.com
    mode: npm
    self_url: https://npm.example.com
```

### Proxy Another npm Mirror

Using npmmirror as an example:

```yaml
sites:
  - id: npm
    host: npm.example.com
    mode: npm
    self_url: https://npm.example.com
    settings:
      upstream_registry_url: https://registry.npmmirror.com
```

## Client Usage

### npm

```bash
npm install lodash --registry=https://npm.example.com
```

Or in `.npmrc`:

```ini
registry=https://npm.example.com
```

### pnpm

```bash
pnpm add lodash --registry=https://npm.example.com
```

### yarn

```bash
yarn config set npmRegistryServer https://npm.example.com
```
