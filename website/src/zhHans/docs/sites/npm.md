---
title: npm 镜像
order: 7
---

# npm 镜像（npm）

## 简介

`npm` 模式将 Pavonis 作为 npm registry 的只读镜像代理，自动重写包元数据中的下载链接，使 `npm`、`pnpm`、`yarn` 等工具的元数据查询与包下载都经过 Pavonis。

该模式不支持发布、登录等写操作。

::: warning self_url 必填
`npm` 模式要求必须配置 `self_url`，格式为 `https://你的域名`（不含路径，不含尾部 `/`）。
:::

## 支持的访问路径

Pavonis 默认放行安装与校验所需的只读接口，其余路径将返回 `404`：

| 路径格式 | 说明 |
|----------|------|
| `/{package}` | 包元数据 |
| `/{package}/{version-or-tag}` | 指定版本或标签的元数据 |
| `/{package}/-/{file}.tgz` | 包压缩文件下载 |
| `/-/package/{package}/dist-tags` | 包的 dist-tags |
| `/-/npm/v1/attestations/{package}@{version}` | 包来源证明（attestations） |
| `/-/npm/v1/keys` | 签名公钥 |
| `/-/ping` | 健康检查 |

以下接口受配置开关控制，关闭时返回 `403`：

| 路径 | 控制字段 | 说明 |
|------|----------|------|
| `/-/v1/search` | `allow_search` | 包搜索 |
| `/-/npm/v1/security/advisories/bulk` | `allow_audit` | 安全审计（`npm audit`） |

## 配置项（settings）

> 对应 Go 配置类型：`NpmSiteSettings`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `upstream_registry_url` | `string` | `https://registry.npmjs.org` | 上游 npm registry 地址（不含尾部 `/`） |
| `allow_search` | `bool` | `false` | 是否允许包搜索接口（`/-/v1/search`） |
| `allow_audit` | `bool` | `true` | 是否允许安全审计接口（`/-/npm/v1/security/advisories/bulk`） |

## 配置示例

### 代理官方 npm registry（默认）

```yaml
sites:
  - id: npm
    host: npm.example.com
    mode: npm
    self_url: https://npm.example.com
```

### 代理其他 npm 镜像源

以 npmmirror 为例：

```yaml
sites:
  - id: npm
    host: npm.example.com
    mode: npm
    self_url: https://npm.example.com
    settings:
      upstream_registry_url: https://registry.npmmirror.com
```

## 客户端使用方式

### npm

```bash
npm install lodash --registry=https://npm.example.com
```

或在 `.npmrc` 中配置：

```ini
registry=https://npm.example.com
```

### pnpm

```bash
pnpm add lodash --registry=https://npm.example.com
```

### yarn

```bash
yarn config set npmRegistryServer https://npm.example.com
```
