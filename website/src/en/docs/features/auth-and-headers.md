---
title: Auth, Headers and Outbound Proxy
order: 3
---

# Auth, Headers and Outbound Proxy

## Container Registry Authentication

The `container_registry_single` and `container_registry_any` modes support enabling **client authentication**, requiring visitors to supply a username and password (HTTP Basic Auth).

### Configuration Example

```yaml
sites:
  - host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      auth:
        enabled: true
        users:
          - name: alice
            password: secret123
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 60s
```

`users` (inline) and `users_file` (external file) can be used together; users from both sources are merged and all are valid.

### Hot-Reload of User Files

`users_file` points to an external YAML file in the same format as the inline `users` list:

```yaml
# users.yml
users:
  - name: alice
    password: secret123
  - name: bob
    password: password456
```

Pavonis reloads this file periodically according to `users_file_reload_interval` — **no restart required** to apply user list changes.

### Forwarding Upstream Credentials

If you need to pass credentials to the upstream registry after authentication, use `$` in the username to separate the Pavonis username from the upstream username:

```
pavonis_user$upstream_user
```

The client logs in to Pavonis as `pavonis_user`; Pavonis forwards requests to the upstream using `upstream_user`'s identity.

See [`ContainerRegistryAuthConfig`](../config/auth) for full details.

## Header Management

### Automatic Cleanup

Before forwarding a request to an upstream, Pavonis automatically removes headers that could expose proxy chain information:

- **Reverse proxy headers**: `Via`, `X-Forwarded-For`, etc.
- **Cloudflare-injected headers**: `CF-Connecting-IP`, `CF-Ray`, `CF-Visitor` and all other `CF-*` headers

This prevents upstreams from detecting the presence of Pavonis.

### Custom Modification

In addition to automatic cleanup, you can explicitly configure **add, modify or delete** operations for any request or response header:

```yaml
request:
  header:
    modify:
      X-Custom-Token: 'my-token'      # add or overwrite request header
    delete:
      - 'X-Internal-Header'           # delete specific request header

response:
  header:
    modify:
      Access-Control-Allow-Origin: '*'
    delete:
      - 'X-Powered-By'
```

See [`RequestConfig`](../config/request) and [`ResponseConfig`](../config/response) for full details.

## Outbound Proxy

A single HTTP outbound proxy can be specified for all upstream requests. Both `http://` and `socks5://` protocols are supported:

```yaml
request:
  proxy: http://127.0.0.1:7890
```

All upstream requests forwarded by Pavonis will go through this proxy, making it suitable for deployments in network-restricted environments.

See [`RequestConfig`](../config/request) for full details.
