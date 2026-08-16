---
title: ResponseConfig (响应设置)
order: 3
---

# ResponseConfig (响应设置)

`response` 部分配置 Pavonis 处理上游响应时的行为，包括最大重定向次数和响应头修改。

## 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_redirects` | `int` | `10` | 当需要跟随重定向时，Pavonis 最多跟随的重定向次数 |
| `header` | `HeaderModificationConfig` | 见下方 | 响应头修改配置，[详见下方](#headermodificationconfig-响应头修改) |

## HeaderModificationConfig (响应头修改)

控制 Pavonis 返回给客户端的响应头。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `modify` | `map[string]string` | `{}` | 添加或覆盖指定响应头 |
| `delete` | `[]string` | `[]` | 删除指定响应头 |

## 配置示例

### 减少最大重定向次数

```yaml
response:
  max_redirects: 3
```

### 添加跨域头部

```yaml
response:
  header:
    modify:
      Access-Control-Allow-Origin: '*'
      Access-Control-Allow-Methods: 'GET, HEAD'
```

### 删除上游返回的特定响应头

```yaml
response:
  header:
    delete:
      - 'X-Powered-By'
      - 'Server'
```
