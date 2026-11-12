---
title: Auth, Headers and Request Proxy
order: 3
---

# Auth, Headers and Request Proxy

## Client Authentication

The `container_registry_single`, `container_registry_any` and `any_proxy` modes can enable client authentication, requiring visitors to supply a username and password (HTTP Basic Auth). Users can be defined directly in the config file or in an external user file. The user file is reloaded periodically, so the user list can be updated without restarting Pavonis.

In the container registry modes, when accessing a private upstream registry that requires authentication, the client can include the upstream credentials alongside its own.

For details, see [ClientAuthConfig](../config/auth).

## Header Management

Before forwarding a request to an upstream, Pavonis removes reverse proxy headers such as `Via` and `X-Forwarded-For`, and headers injected by Cloudflare, by default. This prevents the upstream from learning about the proxy chain. Request and response headers can also be added, modified or deleted as needed.

For the default deletion list and how to customise it, see [RequestConfig](../config/request#headermodificationconfig-request-headers) and [ResponseConfig](../config/response).

## Request Proxy

Upstream requests can go through one or more request proxies, supporting `http`, `https`, `socks5` and `socks5h`. When a proxy is unreachable, Pavonis switches to the next one, and can fall back to a direct connection when all proxies are unavailable. This suits deployments in network-restricted environments.

```text
servers:  proxy1 ── blocked, skipped
          proxy2 ── unreachable, try next
          proxy3 ── ✓ reach upstream via proxy3

all unavailable ──► direct_fallback enabled ? connect directly : return error
```

For configuration, see [RequestConfig](../config/request#requestproxyconfig).
