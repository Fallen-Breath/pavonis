---
title: 测速
order: 7
---

# 测速（speed_test）

## 简介

`speed_test` 模式提供简单的上传/下载测速端点，可用于测试客户端到 Pavonis 之间的网络带宽。

## 测速接口

### 下载测速

发送 `GET` 请求，并通过查询参数 `bytes` 指定要下载的字节数：

```
GET /{path_prefix}?bytes=10485760
```

Pavonis 会返回指定大小的零填充数据（`application/octet-stream`）。

### 上传测速

发送带有请求体的请求（如 `POST`），Pavonis 会读取并丢弃上传的数据：

```
POST /{path_prefix}
Content-Length: 10485760
[request body]
```

## 配置项（settings）

> 对应 Go 配置类型：`SpeedTestSettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_download_bytes` | `int` | `1073741824`（1 GiB） | 单次下载允许的最大字节数。设为 `-1` 禁用下载测速 |
| `max_upload_bytes` | `int` | `1073741824`（1 GiB） | 单次上传允许的最大字节数。设为 `-1` 禁用上传测速 |

## 配置示例

### 基础用法

```yaml
sites:
  - host: speed.example.com
    mode: speed_test
    path_prefix: /speedtest
```

### 限制测速大小

```yaml
sites:
  - host: speed.example.com
    mode: speed_test
    path_prefix: /speedtest
    settings:
      max_download_bytes: 104857600   # 最多下载 100 MiB
      max_upload_bytes: 52428800      # 最多上传 50 MiB
```

### 与其他站点共用主机名

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi

  - host: proxy.example.com
    mode: speed_test
    path_prefix: /speedtest
```

## 使用示例

```bash
# 下载测速（下载 100 MiB）
curl -o /dev/null https://speed.example.com/speedtest?bytes=104857600

# 上传测速（上传 50 MiB 零数据）
dd if=/dev/zero bs=1M count=50 | curl -X POST \
  --data-binary @- \
  https://speed.example.com/speedtest
```
