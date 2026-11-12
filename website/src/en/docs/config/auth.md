---
title: ClientAuthConfig
order: 6
---

# ClientAuthConfig

`auth` is an authentication config block inside the `settings` of the `container_registry_single`, `container_registry_any` and [`any_url`](../sites/any-url) modes. When enabled, clients must provide valid credentials to use the site.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `false` | Whether to enable client authentication |
| `users` | `[]User` | `[]` | Inline users, each with `name` and `password` |
| `users_file` | `string` | none | Path to an external YAML user file, merged with `users` |
| `users_file_reload_interval` | `duration` | none | Periodic hot-reload interval for the external user file. If omitted, the file is only loaded once at startup |

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

## Client Login

In the container registry modes, clients log in to the site with `docker login`. For how to pass credentials in the `any_url` mode, see [Any URL Proxy](../sites/any-url#authentication).

Once the user list is updated (by reloading the user file or restarting), deleted users and old passwords stop working immediately, and so do the tokens clients already hold.

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

Pavonis credentials are never sent to the upstream. The `container_registry_any` mode does not support upstream credentials.

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
