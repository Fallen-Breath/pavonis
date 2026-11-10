---
title: Speed Test
order: 7
---

# Speed Test (speed_test)

::: warning Experimental
This mode is meant for ad-hoc testing. Its behavior and settings may change.
:::

## Overview

The `speed_test` mode provides simple upload/download speed test endpoints for measuring network bandwidth between a client and Pavonis.

Speed test sites are not subject to the traffic or request rate limits in [`resource_limit`](../config/resource-limit). The amount of data per test can be capped with `max_download_bytes` and `max_upload_bytes`.

## Endpoints

### Download Speed Test

Send a `GET` request with the `bytes` query parameter specifying how many bytes to download:

```
GET /speedtest?bytes=10485760
```

Here `/speedtest` is the site's configured `path_prefix`.

Pavonis returns filler data (`application/octet-stream`) of the specified size.

### Upload Speed Test

Send a request with a body (e.g. `POST`); Pavonis reads and discards the uploaded data:

```
POST /speedtest
Content-Length: 10485760
[request body]
```

## Settings

> Corresponding Go config type: `SpeedTestSettings`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_download_bytes` | `int` | `1073741824` (1 GiB) | Maximum bytes allowed per download request. Set to `-1` to disable download testing |
| `max_upload_bytes` | `int` | `1073741824` (1 GiB) | Maximum bytes allowed per upload request. Set to `-1` to disable upload testing |

## Configuration Examples

### Basic Usage

```yaml
sites:
  - host: speed.example.com
    mode: speed_test
    path_prefix: /speedtest
```

### Limit Test Size

```yaml
sites:
  - host: speed.example.com
    mode: speed_test
    path_prefix: /speedtest
    settings:
      max_download_bytes: 104857600   # max 100 MiB download
      max_upload_bytes: 52428800      # max 50 MiB upload
```

### Share a Host with Other Sites

```yaml
sites:
  - host: proxy.example.com
    mode: pypi
    path_prefix: /pypi

  - host: proxy.example.com
    mode: speed_test
    path_prefix: /speedtest
```

## Usage Examples

```bash
# Download speed test (download 100 MiB)
curl -o /dev/null https://speed.example.com/speedtest?bytes=104857600

# Upload speed test (upload 50 MiB of zero data)
dd if=/dev/zero bs=1M count=50 | curl -X POST \
  --data-binary @- \
  https://speed.example.com/speedtest
```
