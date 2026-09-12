---
title: 任意上游代理
order: 0
---

# 任意上游代理（`any_proxy`）

`any_proxy` 会把请求路径中的 URL 作为上游地址进行反向代理。默认不限制上游域名；配置 `domain_blacklist` 后才会拒绝匹配的域名。

## 请求格式

```text
/{上游 URL}
```

URL 可以省略协议。省略时默认使用 `https`：

```text
https://proxy.example.com/https://example.com/file.zip
https://proxy.example.com/example.com/file.zip
```

上游 URL 中的用户名和密码会作为 BasicAuth 转发给上游，与 Pavonis 自身鉴权相互独立：

```text
/https://download-user:download-password@example.com/file.zip
```

## 配置项

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `allowed_methods` | `[]string` | `GET` | 允许的 HTTP 方法 |
| `domain_blacklist` | `[]string` | `[]` | 要拒绝的精确域名或 `*.example.com` 子域名模式 |
| `auth` | `object` | 关闭 | Pavonis 自身 BasicAuth 配置 |

代理请求前会检查域名黑名单。`domain_blacklist` 为空时允许所有域名。`*.example.com` 不包含 `example.com` 本身。

## 配置示例

```yaml
sites:
  - id: anyproxy
    host: proxy.example.com
    mode: any_proxy
    path_prefix: /proxy
    settings:
      allowed_methods: [GET, HEAD]
      domain_blacklist:
        - private.example.com
        - "*.internal.example.com"
      auth:
        enabled: true
        users:
          - name: alice
            password: change-me
```
