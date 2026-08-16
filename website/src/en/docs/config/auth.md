---
title: ContainerRegistryAuthConfig
order: 6
---

# ContainerRegistryAuthConfig

`auth` is an authentication config block inside the `settings` of `container_registry_single` and `container_registry_any` modes. When enabled, clients must provide valid credentials to access container registries through Pavonis.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `false` | Whether to enable client authentication |
| `users` | `[]User` | `[]` | Inline user list; each entry has `name` and `password` |
| `users_file` | `string` | none | Path to an external user file (YAML format); merged with `users` |
| `users_file_reload_interval` | `duration` | none | Periodic hot-reload interval for the external user file. If omitted, the file is only loaded once at startup. Minimum value must be greater than `1s` |

### Username/Password Constraints

- `name` and `password` must not be empty
- `name` and `password` must not contain the characters `$` or `:`

## Inline Users (users)

Define users directly in the config file:

```yaml
settings:
  auth:
    enabled: true
    users:
      - name: alice
        password: alice-secret
      - name: bob
        password: bob-secret
```

## External User File (users_file)

Store the user list in a separate YAML file for easier management:

**`users.yml` format:**

```yaml
users:
  - name: alice
    password: alice-secret
  - name: bob
    password: bob-secret
```

**Reference in config:**

```yaml
settings:
  auth:
    enabled: true
    users_file: /etc/pavonis/users.yml
```

### Hot Reload

With `users_file_reload_interval` set, Pavonis periodically re-reads the user file in the background — no restart required to apply added or removed users:

```yaml
settings:
  auth:
    enabled: true
    users_file: /etc/pavonis/users.yml
    users_file_reload_interval: 5m    # reload every 5 minutes
```

When both `users` and `users_file` are configured, both lists are merged and all users are valid.

## How Authentication Works

When auth is enabled, Pavonis rewrites the `realm` URL in the OCI standard authentication flow to point to its own `/auth` endpoint. The client (e.g. `docker login`) authenticates as follows:

1. Client requests `/v2/` → receives a `401` response with `WWW-Authenticate` containing `realm=https://your-pavonis/auth`
2. Client sends credentials to Pavonis's `/auth` endpoint
3. Pavonis validates the credentials; if valid, it makes a real token request to the upstream and returns the token to the client
4. Client uses the token to continue requesting image data

### Forwarding Upstream Credentials (Advanced)

When the upstream registry itself requires authentication (e.g. a private GHCR repo), the client can pass **both Pavonis credentials and upstream credentials** simultaneously using `$` as a separator:

```
Username format:  pavonis_username$upstream_username
Password format:  pavonis_password$upstream_password
```

Example with Docker CLI:

```bash
docker login docker.example.com \
  -u "alice\$upstream-user" \
  -p "alice-secret\$upstream-token"
```

If only Pavonis credentials are provided (no `$`), Pavonis makes anonymous requests to the upstream (suitable when the upstream registry is public).

## Full Configuration Examples

### Single Registry Proxy with Auth + Hot Reload

```yaml
sites:
  - id: dockerhub
    host: docker.example.com
    mode: container_registry_single
    self_url: https://docker.example.com
    settings:
      auth:
        enabled: true
        users:
          - name: admin
            password: admin-password
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 10m
```

### Any-Registry Proxy with Auth

```yaml
sites:
  - id: mirror
    host: mirror.example.com
    mode: container_registry_any
    self_url: https://mirror.example.com
    settings:
      auth:
        enabled: true
        users_file: /etc/pavonis/users.yml
        users_file_reload_interval: 5m
```
