---
title: PyPI 镜像
order: 5
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

> 对应 Go 配置类型：`PypiRegistrySettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `upstream_simple_url` | `string` | `https://pypi.org/simple` | 上游 Simple API 地址（不含尾部 `/`）|
| `upstream_files_url` | `string` | `https://files.pythonhosted.org` | 上游文件下载地址（不含尾部 `/`）|

::: tip
`upstream_simple_url` 和 `upstream_files_url` 必须同时设置或同时不设置，不能只设置其中一个。
:::

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

### pyproject.toml（uv / Poetry）

```toml
[[tool.uv.index]]
url = "https://pypi.example.com/simple/"
default = true
```
