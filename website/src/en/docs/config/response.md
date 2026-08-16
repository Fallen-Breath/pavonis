---
title: ResponseConfig
order: 3
---

# ResponseConfig

The `response` section configures how Pavonis processes upstream responses, including the maximum number of redirects to follow and response header modification.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_redirects` | `int` | `10` | Maximum number of redirects Pavonis will follow when redirect-following is required |
| `header` | `HeaderModificationConfig` | see below | Response header modification config, [see below](#headermodificationconfig-response-headers) |

## HeaderModificationConfig (Response Headers)

Controls the response headers returned to clients.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `modify` | `map[string]string` | `{}` | Add or overwrite specified response headers |
| `delete` | `[]string` | `[]` | Delete specified response headers |

## Configuration Examples

### Reduce Max Redirects

```yaml
response:
  max_redirects: 3
```

### Add CORS Headers

```yaml
response:
  header:
    modify:
      Access-Control-Allow-Origin: '*'
      Access-Control-Allow-Methods: 'GET, HEAD'
```

### Remove Specific Upstream Response Headers

```yaml
response:
  header:
    delete:
      - 'X-Powered-By'
      - 'Server'
```
