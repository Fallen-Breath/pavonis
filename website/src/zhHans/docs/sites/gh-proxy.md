---
title: GitHub 文件加速
order: 1
---

# GitHub 文件加速（gh_proxy）

## 简介

`gh_proxy` 模式将 Pavonis 作为 GitHub 文件下载的反向代理，用于加速访问 GitHub 上的各类资源，适合网络访问 GitHub 受限的场景。

**支持代理的上游主机：**

| 主机 | 支持的路径 |
|------|-----------|
| `github.com` | Releases、Archive、Raw、Blob、git-upload-pack |
| `raw.githubusercontent.com` | Raw 文件内容 |
| `gist.github.com` | Gist 页面 |
| `gist.githubusercontent.com` | Gist 原始文件 |

## 请求格式

客户端请求路径格式为：

```
/{完整的 GitHub URL}
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
| `size_limit` | `int` | `0` | 响应体大小上限（字节），`0` 表示不限制。超过限制时返回 `502` |
| `raw_text_url_rewrite` | `bool` | `false` | 是否重写 Raw 文本文件内容中的 GitHub URL，使其指向本代理。**启用时 `self_url` 必填** |
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

开启后，`raw.githubusercontent.com` 和 `gist.githubusercontent.com` 返回的文本文件中，若内容包含指向这些主机的 URL，会被自动替换为指向本代理的地址。

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
