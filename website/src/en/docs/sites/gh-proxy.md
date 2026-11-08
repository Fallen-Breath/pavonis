---
title: GitHub Asset Acceleration
order: 1
---

# GitHub Asset Acceleration (gh_proxy)

## Overview

The `gh_proxy` mode turns Pavonis into a reverse proxy for GitHub file downloads, accelerating access to various GitHub resources. It is ideal for environments where direct access to GitHub is restricted.

**Upstream hosts proxied:**

| Host | Supported Paths |
|------|----------------|
| `github.com` | Releases, Archive, Raw, Blob, git-upload-pack |
| `raw.githubusercontent.com` | Raw file content |
| `gist.github.com` | Gist pages |
| `gist.githubusercontent.com` | Gist raw files |

## Request Format

Client request paths follow the format:

```
/{GitHub URL (scheme optional)}
```

The upstream URL scheme may be omitted; Pavonis defaults it to `https`:

```
https://gh.example.com/https://github.com/user/repo/releases/download/v1.0/app.tar.gz
https://gh.example.com/github.com/user/repo/releases/download/v1.0/app.tar.gz
```

For example, if the `gh_proxy` site is bound to `gh.example.com`:

```
# Download a GitHub Release asset
https://gh.example.com/https://github.com/user/repo/releases/download/v1.0/app.tar.gz

# Access a Raw file
https://gh.example.com/https://raw.githubusercontent.com/user/repo/main/README.md

# Clone a repository (client must prefix URLs)
git clone https://gh.example.com/https://github.com/user/repo.git
```

## Settings

> Corresponding Go config type: `GithubDownloadProxySettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `size_limit` | `int` | `0` | Maximum response body size in bytes. `0` means unlimited. Requests exceeding the limit return `502` |
| `raw_text_url_rewrite` | `bool` | `false` | Whether to rewrite GitHub URLs found inside Raw text file content so they point to this proxy. **Requires `self_url` when enabled** |
| `repos_whitelist` | `[]string` | `[]` | Repository allowlist. Requests for repos not on the list return `403` |
| `repos_blacklist` | `[]string` | `[]` | Repository denylist. Requests for repos on the list return `403` |

### Allow/Deny List Format

Each entry is in the form `owner/repo`, with `*` wildcards supported:

```yaml
repos_whitelist:
  - 'torvalds/linux'      # exact match
  - 'torvalds/*'          # all repos under torvalds
  - '*/linux'             # all repos named linux
```

When both lists are configured, a repository must be on the allowlist and not on the denylist for the request to pass.

## Configuration Examples

### Basic Usage

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
```

### Limit Download Size

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
    settings:
      size_limit: 104857600   # 100 MiB
```

### Enable Raw Text URL Rewriting

When enabled, Pavonis rewrites the URLs in a UTF-8 text file from `raw.githubusercontent.com` or `gist.githubusercontent.com` that point to these two hosts, so they go through this proxy. Downloads the client then makes based on the file content still pass through this proxy.

For example, a client downloads an install script through the proxy:

```bash
curl -fsSL https://gh.example.com/https://raw.githubusercontent.com/user/repo/main/install.sh
```

The script's content on GitHub is as follows. The line that downloads `setup.conf` points to `raw.githubusercontent.com`:

```sh
#!/bin/sh
set -e

echo "Installing example tool..."
curl -fsSL https://raw.githubusercontent.com/user/repo/main/setup.conf -o setup.conf
mkdir -p /etc/example
mv setup.conf /etc/example/
echo "Done."
```

When disabled, the client receives exactly this script. When it reaches the `setup.conf` download, it connects to GitHub directly and fails on networks where GitHub is unreachable.

When enabled, in the script the client receives, that download URL has been rewritten to go through this proxy:

```sh
#!/bin/sh
set -e

echo "Installing example tool..."
curl -fsSL https://gh.example.com/https://raw.githubusercontent.com/user/repo/main/setup.conf -o setup.conf
mkdir -p /etc/example
mv setup.conf /etc/example/
echo "Done."
```

This way users do not need to edit the download URLs in the script by hand, and no separate copy of the script has to be maintained on GitHub for the proxy.

Rewriting modifies the file content, so it is not suitable for files whose hash or signature is verified.

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
    self_url: https://gh.example.com
    settings:
      raw_text_url_rewrite: true
```

### Configure an Allowlist

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
    settings:
      repos_whitelist:
        - 'my-org/*'
        - 'trusted-user/specific-repo'
```
