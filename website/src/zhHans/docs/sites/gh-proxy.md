---
title: GitHub 文件加速
order: 1
---

# GitHub 文件加速（gh_proxy）

## 简介

`gh_proxy` 模式将 Pavonis 作为 GitHub 文件下载的反向代理，用于加速访问 GitHub 上的各类资源，适合网络访问 GitHub 受限的场景。

**支持代理的上游主机与路径：**

| 主机 | 支持的路径 |
|------|-----------|
| `github.com` | Release 附件（`releases/download/`、`releases/latest/download/`）、源码压缩包（`archive/`）、Raw 文件（`raw/`）、文件页面（`blob/`，返回文件原始内容）、git clone / fetch / push、git LFS |
| `raw.githubusercontent.com` | Raw 文件内容 |
| `gist.github.com` | gist 原始文件（`raw`）、gist 压缩包（`archive/`、`download`）、git clone / push |
| `gist.githubusercontent.com` | gist 原始文件（`raw`） |

不在上表中的路径，例如仓库主页、Release 页面、Issue 等网页，一律返回 `404`。

git push 与 git LFS 锁操作需开启 `allow_push`，否则返回 `403`。

访问私有仓库或执行 push 时，git 会提示输入 GitHub 凭据，凭据经 Pavonis 转发给 GitHub。

git LFS 只代理对象的查询请求，LFS 对象本身仍由客户端直接与 GitHub 传输。

## 请求格式

客户端请求路径格式为：

```
/{GitHub URL（协议可选）}
```

上游 URL 可以省略协议；省略时 Pavonis 默认使用 `https`：

```
https://gh.example.com/https://github.com/user/repo/releases/download/v1.0/app.tar.gz
https://gh.example.com/github.com/user/repo/releases/download/v1.0/app.tar.gz
```

例如，若 Pavonis 的 `gh_proxy` 站点绑定在 `gh.example.com`，则：

```
# 下载 GitHub Release 文件
https://gh.example.com/https://github.com/user/repo/releases/download/v1.0/app.tar.gz

# 访问 Raw 文件
https://gh.example.com/https://raw.githubusercontent.com/user/repo/main/README.md

# 克隆仓库（需要客户端设置代理前缀）
git clone https://gh.example.com/https://github.com/user/repo.git
```

## 配置项（settings）

> 对应 Go 配置类型：`GithubDownloadProxySettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `size_limit` | `int` | `0` | 请求体与响应体各自的大小上限（字节），`0` 表示不限制。超过上限的请求或响应会被拒绝；若无法提前得知大小，则在传输超过上限时中断 |
| `raw_text_url_rewrite` | `bool` | `false` | 是否重写 Raw 文本文件内容中的 GitHub URL，使其指向本代理。**启用时 `self_url` 必填** |
| `allow_push` | `bool` | `false` | 是否允许 git push 与 git LFS 锁操作 |
| `repos_whitelist` | `[]string` | `[]` | 仓库白名单，不在白名单内的仓库请求将返回 `403` |
| `repos_blacklist` | `[]string` | `[]` | 仓库黑名单，在黑名单内的仓库请求将返回 `403` |

### 白黑名单格式

白黑名单列表中每项格式为 `作者/仓库名`，支持通配符 `*`：

```yaml
repos_whitelist:
  - 'torvalds/linux'      # 精确匹配
  - 'torvalds/*'          # 匹配 torvalds 下所有仓库
  - '*/linux'             # 匹配所有名为 linux 的仓库
```

同时配置白名单和黑名单时，仓库须在白名单内且不在黑名单内，请求才会被放行。

匹配不区分大小写，仓库名带不带 `.git` 后缀视为同一仓库。

gist 只按作者匹配，名单条目需写成 `作者/*`。gist 页面默认给出的克隆地址 `https://gist.github.com/{gist ID}.git` 中不含作者，配置了任一名单时，这类请求会返回 `403`，需改用带作者用户名的地址 `https://gist.github.com/{作者}/{gist ID}.git`。

## 配置示例

### 基础用法

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
```

### 限制下载大小

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
    settings:
      size_limit: 104857600   # 100 MiB
```

### 启用 Raw 文本 URL 重写

开启后，Pavonis 返回 `raw.githubusercontent.com` 和 `gist.githubusercontent.com` 的 UTF-8 文本文件时，会把文件内容中本代理支持的 GitHub URL（见[简介](#简介)中的主机与路径）改写为经由本代理的 URL。客户端据此继续下载时，请求仍然经过本代理。

例如，客户端通过代理下载一个安装脚本：

```bash
curl -fsSL https://gh.example.com/https://raw.githubusercontent.com/user/repo/main/install.sh
```

该脚本在 GitHub 上的内容如下，其中下载 `setup.conf` 的一行指向 `raw.githubusercontent.com`：

```sh
#!/bin/sh
set -e

echo "Installing example tool..."
curl -fsSL https://raw.githubusercontent.com/user/repo/main/setup.conf -o setup.conf
mkdir -p /etc/example
mv setup.conf /etc/example/
echo "Done."
```

未开启时，客户端收到的脚本与上面完全相同，脚本运行到下载 `setup.conf` 时会直连 GitHub，在无法访问 GitHub 的网络中失败。

开启后，客户端收到的脚本中，该下载地址已被改写为经由本代理：

```sh
#!/bin/sh
set -e

echo "Installing example tool..."
curl -fsSL https://gh.example.com/https://raw.githubusercontent.com/user/repo/main/setup.conf -o setup.conf
mkdir -p /etc/example
mv setup.conf /etc/example/
echo "Done."
```

这样使用者不必手动修改脚本中的下载地址，GitHub 上也无需为代理单独维护一份脚本。

改写会修改文件内容，因此不适用于需要校验哈希或签名的文件。

```yaml
sites:
  - id: ghproxy
    host: gh.example.com
    mode: gh_proxy
    self_url: https://gh.example.com
    settings:
      raw_text_url_rewrite: true
```

### 配置白名单

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
