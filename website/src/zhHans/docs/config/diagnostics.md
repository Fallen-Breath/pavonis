---
title: DiagnosticsConfig (诊断服务)
order: 7
---

# DiagnosticsConfig (诊断服务)

`diagnostics` 部分配置 Pavonis 的内部诊断服务，该服务在一个独立的端口上提供自检接口，与主代理服务分离。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | `bool` | `false` | 是否启用诊断服务 |
| `listen` | `string` | `127.0.0.1:6009` | 诊断服务的监听地址 |

## 配置示例

### 启用诊断服务

```yaml
diagnostics:
  enabled: true
```

### 自定义监听地址

```yaml
diagnostics:
  enabled: true
  listen: 127.0.0.1:9090
```

## 注意事项

- 诊断服务默认只监听 `127.0.0.1`，不对外暴露，避免泄露内部信息。
- 如需在容器环境中访问诊断服务，可将监听地址改为 `0.0.0.0:6009`，并通过防火墙或网络策略限制访问。
