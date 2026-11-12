---
title: Any Upstream Proxy
order: 0
---

# Any Upstream Proxy (`any_proxy`)

`any_proxy` forwards requests to an upstream URL embedded in the request path. Any public domain is allowed by default, but private network addresses are rejected.

## Request format

```text
/{upstream URL}
```

If the scheme is omitted, Pavonis uses `https`:

```text
https://proxy.example.com/https://example.com/file.zip
https://proxy.example.com/example.com/file.zip
```

## Authentication

The `Authorization` request header is only used for Pavonis's own [client authentication](../config/auth), and is never forwarded to the upstream. Tools that cannot set request headers can put the credentials in the Pavonis address, e.g. use `https://alice:change-me@proxy.example.com/` as the proxy prefix.

If the upstream requires authentication, provide its credentials in either of the following ways, but not both:

- In the upstream URL, sent to the upstream as Basic Auth: `/https://user:password@example.com/file.zip`. Note that credentials in the URL may show up in the access logs of Pavonis and its front proxies
- In the `X-Pavonis-Upstream-Authorization` request header, which Pavonis sends to the upstream as `Authorization`. Any authentication scheme works:

  ```bash
  curl -u alice:change-me \
    -H "X-Pavonis-Upstream-Authorization: Bearer <token>" \
    https://proxy.example.com/https://api.example.com/file.zip
  ```

The `WWW-Authenticate` header of upstream responses is renamed to `X-Pavonis-Upstream-WWW-Authenticate`, so that it is not mistaken for Pavonis's own authentication challenge.

## Settings

| Field | Type | Default | Description |
|---|---|---|---|
| `allowed_methods` | `[]string` | `[GET, HEAD]` | HTTP methods accepted by this site. `git clone` and the like need `POST` |
| `domain_whitelist` | `[]string` | `[]` | Exact domains or `*.example.com` subdomain patterns to allow. Empty means no restriction |
| `domain_blacklist` | `[]string` | `[]` | Exact domains or `*.example.com` subdomain patterns to reject |
| `allow_private_network` | `bool` | `false` | Whether private network addresses can be accessed |
| `size_limit` | `int` | `0` | Maximum size in bytes, applied to the request body and the response body separately. `0` means unlimited |
| `auth` | [`ClientAuthConfig`](../config/auth) | disabled | Client authentication (HTTP Basic Auth) |

## Access restrictions

::: warning
Without `auth` or `domain_whitelist`, anyone can use the site to access **any** website.
:::

In the domain lists, `*.example.com` only matches subdomains, not `example.com` itself.

Private network addresses include loopback, private ranges, link-local addresses, etc. Domains resolving to them are rejected too. However, when the upstream is accessed through a [request proxy](../config/request), domains are resolved by the proxy server, so Pavonis can only reject private IPs and `localhost` written literally. Restrict it on the proxy server if needed.

All of these checks also apply to every redirect returned by the upstream. Redirecting to a rejected target results in `403`.

## Example

```yaml
sites:
  - id: anyproxy
    host: proxy.example.com
    mode: any_proxy
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
