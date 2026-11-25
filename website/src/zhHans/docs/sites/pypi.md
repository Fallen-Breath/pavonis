---
title: PyPI 镜像
order: 6
---

# PyPI 镜像（pypi）

## 简介

`pypi` 模式将 Pavonis 作为 PyPI 包索引的镜像代理，自动重写响应中的下载链接，使 `pip`、`poetry`、`uv` 等工具的所有请求（包括索引查询与文件下载）都经过 Pavonis。

## 路由规则

| 请求路径 | 转发到 |
|----------|--------|
| `/simple/...` | `upstream_simple_url`（包索引接口） |
| `/files/...` | `upstream_files_url`（包文件下载） |

Pavonis 会自动将上游 Simple API 响应（HTML 和 JSON 格式）中的下载链接重写为指向本站 `/files/...` 路径，客户端无需感知实际的上游文件服务器地址。

## 配置项（settings）

> 对应 Go 配置类型：`PypiSiteSettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `upstream_simple_url` | `string` | `https://pypi.org/simple` | 上游 Simple API 地址（不含尾部 `/`）|
| `upstream_files_url` | `string` | `https://files.pythonhosted.org` | 上游文件下载地址（不含尾部 `/`）|
| `packages_whitelist` | `[]string` | `[]` | 包白名单，不在白名单内的包请求将返回 `403` |
| `packages_blacklist` | `[]string` | `[]` | 包黑名单，在黑名单内的包请求将返回 `403` |

::: tip
`upstream_simple_url` 和 `upstream_files_url` 必须同时设置或同时不设置，不能只设置其中一个。
:::

### 白黑名单格式

每项填写一个 PyPI 包名，不支持通配符。匹配按 [PEP 503](https://peps.python.org/pep-0503/#normalized-names) 规范化后精确比对，不区分大小写，连续的 `-`、`_`、`.` 视为等价。

同时配置 `packages_whitelist` 和 `packages_blacklist` 时，包须在白名单内且不在黑名单内，请求才会被放行。

配置了任一名单时，`/simple/` 根目录列表仍会放行。`/files/...` 下只有标准的 wheel（`.whl`）和源码包（`.tar.gz`）及其 `.metadata` 文件能够识别包名，其余文件请求会返回 `403`。

## 配置示例

### 代理官方 PyPI（默认）

```yaml
sites:
  - id: pypi
    host: pypi.example.com
    mode: pypi
```

### 代理其他 PyPI 镜像源

以清华大学镜像为例：

```yaml
sites:
  - id: pypi
    host: pypi.example.com
    mode: pypi
    settings:
      upstream_simple_url: https://pypi.tuna.tsinghua.edu.cn/simple
      upstream_files_url: https://pypi.tuna.tsinghua.edu.cn/packages
```

### 结合 path_prefix 使用

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi
```

### 限制允许访问的包

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

## 客户端使用方式

### pip

```bash
pip install requests -i https://pypi.example.com/simple/
```

或在 `pip.conf` 中配置：

```ini
[global]
index-url = https://pypi.example.com/simple/
```

### uv

```bash
uv pip install requests --index-url https://pypi.example.com/simple/
```

或在 `pyproject.toml` 中配置：

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
