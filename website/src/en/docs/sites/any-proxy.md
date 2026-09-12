---
title: Any Upstream Proxy
order: 0
---

# Any Upstream Proxy (`any_proxy`)

`any_proxy` forwards requests to an upstream URL embedded in the request path. It does not restrict upstream hosts unless `domain_blacklist` is configured.

## Request format

```text
/{upstream URL}
```

The scheme may be omitted; Pavonis then defaults to `https`:

```text
https://proxy.example.com/https://example.com/file.zip
https://proxy.example.com/example.com/file.zip
```

Credentials in the upstream URL are forwarded to the upstream as Basic Authentication. They are separate from Pavonis authentication:

```text
/https://download-user:download-password@example.com/file.zip
```

## Settings

| Field | Type | Default | Description |
|---|---|---|---|
| `allowed_methods` | `[]string` | `GET` | HTTP methods accepted by this site |
| `domain_blacklist` | `[]string` | `[]` | Exact domains or `*.example.com` subdomain patterns to reject |
| `auth` | `object` | disabled | Pavonis BasicAuth configuration |

The blacklist is checked before contacting the upstream. Empty `domain_blacklist` allows all domains. A wildcard such as `*.example.com` does not include `example.com` itself.

## Example

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
