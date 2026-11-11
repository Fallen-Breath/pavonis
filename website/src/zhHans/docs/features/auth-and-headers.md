---
title: 认证、请求头与请求代理
order: 3
---

# 认证、请求头与请求代理

## 客户端认证

`container_registry_single`、`container_registry_any` 和 `any_proxy` 模式可以启用客户端认证，访问者需要提供用户名和密码（HTTP Basic Auth）。用户可以直接写在配置文件中，也可以放在外部用户文件里。用户文件支持定期热重载，更新用户列表无需重启 Pavonis。

容器仓库模式下，访问需要认证的私有上游仓库时，客户端可以在凭据中附带上游仓库的凭据。

具体写法见 [ClientAuthConfig](../config/auth)。

## 请求头管理

Pavonis 向上游转发请求前，默认删除 `Via`、`X-Forwarded-For` 等反向代理头部和 Cloudflare 注入的头部，避免上游得知代理链信息。请求头和响应头也可以按需添加、修改或删除。

默认删除的头部列表和自定义写法见 [RequestConfig](../config/request#headermodificationconfig-请求头修改) 与 [ResponseConfig](../config/response)。

## 请求代理

上游请求可以经过一个或多个请求代理，支持 `http`、`https`、`socks5`、`socks5h`。某个代理连不上时自动切换到下一个，也可以在所有代理都不可用时回退到直连。适合在网络访问受限的环境中部署。

配置方式见 [RequestConfig](../config/request#requestproxyconfig)。
