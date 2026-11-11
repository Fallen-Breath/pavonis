---
title: GitHub Asset Acceleration
order: 1
---

# GitHub Asset Acceleration (gh_proxy)

## Overview

The `gh_proxy` mode turns Pavonis into a reverse proxy for GitHub file downloads, accelerating access to various GitHub resources. It is ideal for environments where direct access to GitHub is restricted.

**Upstream hosts and paths proxied:**

| Host | Supported Paths |
|------|----------------|
| `github.com` | Release assets (`releases/download/`, `releases/latest/download/`), source archives (`archive/`), raw files (`raw/`), file pages (`blob/`, served as the raw file content), git clone / fetch / push, git LFS |
| `raw.githubusercontent.com` | Raw file content |
| `gist.github.com` | Gist raw files (`raw`), gist archives (`archive/`, `download`), git clone / push |
| `gist.githubusercontent.com` | Gist raw files (`raw`) |

Paths not listed above, such as repository home pages, release pages and issues, return `404`.

git push and git LFS lock operations require `allow_push`; otherwise they return `403`.

When accessing a private repository or pushing, git prompts for GitHub credentials, which Pavonis forwards to GitHub.

For git LFS, only the object lookup requests are proxied. The LFS objects themselves are still transferred by the client directly with GitHub.

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
| `size_limit` | `int` | `0` | Maximum size in bytes, applied to the request body and the response body separately. `0` means unlimited. Larger requests or responses are refused; if the size is not known in advance, the transfer is aborted once it exceeds the limit |
| `raw_text_url_rewrite` | `bool` | `false` | Whether to rewrite GitHub URLs found inside Raw text file content so they point to this proxy. **Requires `self_url` when enabled** |
| `allow_push` | `bool` | `false` | Whether to allow git push and git LFS lock operations |
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

Matching is case-insensitive, and a repository name with or without the `.git` suffix refers to the same repository.

Gists are matched by owner only, so list entries for gists must be written as `owner/*`. The default clone URL shown on gist pages, `https://gist.github.com/{gist ID}.git`, does not contain the owner. When either list is configured, such requests return `403`; use the clone URL with the owner's username instead: `https://gist.github.com/{owner}/{gist ID}.git`.

## Configuration Examples

### Basic Usage

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
```

### Limit Transfer Size

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
    settings:
      size_limit: 104857600   # 100 MiB
```

### Enable Raw Text URL Rewriting

When enabled, Pavonis rewrites the GitHub URLs supported by this proxy (see the hosts and paths in the [Overview](#overview)) in a UTF-8 text file from `raw.githubusercontent.com` or `gist.githubusercontent.com`, so they go through this proxy. Downloads the client then makes based on the file content still pass through this proxy.

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
