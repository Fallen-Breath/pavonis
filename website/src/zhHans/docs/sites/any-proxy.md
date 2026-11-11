---
title: 任意上游代理
order: 0
---

# 任意上游代理（`any_proxy`）

`any_proxy` 会把请求路径中的 URL 作为上游地址进行反向代理。默认允许任意公网域名，但拒绝访问内网地址。

## 请求格式

```text
/{上游 URL}
```

URL 可以省略协议。省略时默认使用 `https`：

```text
https://proxy.example.com/https://example.com/file.zip
https://proxy.example.com/example.com/file.zip
```

## 认证

`Authorization` 请求头只用于 Pavonis 自身的[客户端认证](../config/auth)，永远不会转发给上游。无法设置请求头的工具，可以把凭据写在 Pavonis 的地址中，例如使用 `https://alice:change-me@proxy.example.com/` 作为代理前缀。

上游需要认证时，可选择以下任一方式提供凭据，但不能同时使用：

- 写在上游 URL 中，以 Basic Auth 形式发给上游：`/https://user:password@example.com/file.zip`。注意 URL 中的凭据可能会出现在 Pavonis 及其前置代理的访问日志中
- 放在 `X-Pavonis-Upstream-Authorization` 请求头中，Pavonis 会将其作为 `Authorization` 发给上游，支持任意认证方式：

  ```bash
  curl -u alice:change-me \
    -H "X-Pavonis-Upstream-Authorization: Bearer <token>" \
    https://proxy.example.com/https://api.example.com/file.zip
  ```

上游响应中的 `WWW-Authenticate` 会改名为 `X-Pavonis-Upstream-WWW-Authenticate` 返回，以免与 Pavonis 自身的认证质询混淆。

## 配置项

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `allowed_methods` | `[]string` | `[GET, HEAD]` | 允许的 HTTP 方法。`git clone` 等需要 `POST` |
| `domain_whitelist` | `[]string` | `[]` | 允许的精确域名或 `*.example.com` 子域名模式。为空时不限制 |
| `domain_blacklist` | `[]string` | `[]` | 要拒绝的精确域名或 `*.example.com` 子域名模式 |
| `allow_private_network` | `bool` | `false` | 是否允许访问内网地址 |
| `size_limit` | `int` | `0` | 请求体与响应体各自的大小上限（字节），`0` 表示不限制 |
| `auth` | [`ClientAuthConfig`](../config/auth) | 关闭 | 客户端认证（HTTP Basic Auth） |

## 访问限制

- 黑白名单和内网检查对上游返回的每一次重定向同样生效，重定向到被拒绝的目标时返回 `403`。
- `*.example.com` 不包含 `example.com` 本身。
- 内网地址包括回环地址、私有网段、链路本地地址（含云服务器的元数据接口 `169.254.169.254`）等。解析到内网地址的域名同样会被拒绝。
- 通过[请求代理](../config/request)访问上游时，域名由代理服务器解析，Pavonis 只能拦截直接写成内网 IP 或 `localhost` 的地址。如有需要，请在代理服务器上限制。
- 既不启用 `auth`，也不配置 `domain_whitelist` 时，任何人都能借助该站点访问任意网站。

## 配置示例

```yaml
sites:
  - id: anyproxy
    host: proxy.example.com
    mode: any_proxy
    path_prefix: /proxy
    settings:
      domain_whitelist:
        - github.com
        - "*.githubusercontent.com"
      size_limit: 1073741824  # 1 GiB
      auth:
        enabled: true
        users:
          - name: alice
            password: change-me
```
