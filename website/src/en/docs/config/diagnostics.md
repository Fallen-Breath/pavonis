---
title: DiagnosticsConfig
order: 7
---

# DiagnosticsConfig

The `diagnostics` section configures Pavonis's internal diagnostics service, which runs on a separate port and is isolated from the main proxy service.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `false` | Whether to enable the diagnostics service |
| `listen` | `string` | `127.0.0.1:6009` | Listen address for the diagnostics service |

## Configuration Examples

### Enable the Diagnostics Service

```yaml
diagnostics:
  enabled: true
```

### Custom Listen Address

```yaml
diagnostics:
  enabled: true
  listen: 127.0.0.1:9090
```

## Notes

- The diagnostics service listens on `127.0.0.1` by default and is not exposed to the internet, preventing leakage of internal information.
- In container environments where you need to access the diagnostics service, you can change the listen address to `0.0.0.0:6009` and restrict access via firewall or network policies.
