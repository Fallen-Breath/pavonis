---
title: 支持的生态
order: 4
---

# 支持的生态

除了内置专用 `mode` 的平台外，许多常见的包管理器与软件仓库也可以通过 [`http`](../sites/http) 等通用模式由 Pavonis 代理。下表列出了经过实测可用的上游生态与对应模式：

| 生态 / 站点 | 模式 | 说明 |
|------------|------|------|
| [Conda](#conda) | [`http`](../sites/http) | 代理 `conda-forge` 通道索引与包下载，兼容 `conda`、`mamba`、`micromamba` |
| [crates.io (Cargo)](#crates-io-cargo) | [`http`](../sites/http) | 代理 Rust sparse 索引与 `.crate` 包下载 |
| [GitHub](../sites/github) | [`github`](../sites/github) | 代理 Release 附件、源码压缩包、Raw 文件、Gist 下载以及 `git clone/fetch/push` |
| [Go Modules](#go-modules) | [`http`](../sites/http) | 代理 `proxy.golang.org` 模块下载与 `sum.golang.org` 校验查询 |
| [Gradle](#gradle) | [`http`](../sites/http) | 代理 Maven Central、Gradle Plugin Portal 与 Wrapper 发行包下载 |
| [Homebrew](#homebrew) | [`http`](../sites/http) + [`container_registry_single`](../sites/container-registry-single) | 代理公式签名 API 与 `homebrew/core` 预编译 bottle 下载 |
| [Hugging Face](../sites/hugging-face) | [`hugging_face`](../sites/hugging-face) | 代理模型与数据集文件下载，兼容 `hf` CLI、`transformers`、`datasets` |
| [Maven Central](#maven-central) | [`http`](../sites/http) | 代理 Maven Central 的 POM、依赖与构建插件下载 |
| [npm](../sites/npm) | [`npm`](../sites/npm) | 代理 npm registry 只读元数据与压缩包下载，兼容 `npm`、`pnpm`、`yarn` |
| [OCI 容器仓库](../sites/container-registry-single) | [`container_registry_single`](../sites/container-registry-single) / [`container_registry_any`](../sites/container-registry-any) | 代理 Docker Hub、GHCR、NVCR 等容器镜像仓库，兼容 `docker`、`containerd`、`crictl` |
| [PyPI](../sites/pypi) | [`pypi`](../sites/pypi) | 代理 Python 包索引与包文件下载，兼容 `pip`、`uv`、`Poetry` |
| [RubyGems](#rubygems) | [`http`](../sites/http) | 代理 Compact Index 与 `.gem` 包下载，兼容 `gem` 与 `Bundler` |

对外提供服务时建议配合 [HTTPS 与反向代理](./https) 配置 HTTPS。如果不想为每个生态单独准备域名，也可以通过站点的 `path_prefix` 或 `http` 模式的 `mappings` 合并到同一域名下。

## Conda

**仅支持在索引中使用相对路径的通道**，例如 `conda-forge`。若某个通道在索引中直接返回外部绝对下载地址，客户端仍会直连该地址。`conda-forge` 会将包下载请求重定向到对象存储，设置 `redirect_action: follow_all` 由 Pavonis 代为跟随：

```yaml
sites:
  - id: conda
    host: conda.example.com
    mode: http
    settings:
      redirect_action: follow_all
      mappings:
        - path: /conda-forge
          destination: https://conda.anaconda.org/conda-forge
```

`conda`、`mamba` 和 `micromamba` 使用相同的命令行参数指定通道：

```bash
micromamba create -n myenv --override-channels -c https://conda.example.com/conda-forge bzip2
```

或在 `~/.condarc` 中配置：

```yaml
channels:
  - https://conda.example.com/conda-forge
```

## crates.io (Cargo)

**仅支持通过 sparse 协议查询包索引与下载 `.crate` 包**，不支持发布、撤回、`cargo search` 等调用 `crates.io` Web API 的操作。

Cargo 从索引根目录的 `config.json` 读取 `dl` 字段来拼接 `.crate` 下载地址。由于上游以 `application/octet-stream` 返回该文件，`body_rewrites` 需要同时匹配 `application/json` 和 `application/octet-stream`，并通过 `json_paths` 将 `dl` 改写到本站的 `/files` 路径：

```yaml
sites:
  - id: cargo
    host: cargo.example.com
    mode: http
    settings:
      redirect_action: follow_all
      mappings:
        - path: /index
          destination: https://index.crates.io
        - path: /files
          destination: https://static.crates.io/crates
      body_rewrites:
        - paths:
            - '/index/config.json'
          content_types:
            - 'application/json'
            - 'application/octet-stream'
          json_paths:
            - 'dl'
          replacements:
            - search: https://static.crates.io/crates
              replace: https://cargo.example.com/files
```

在 `~/.cargo/config.toml` 或项目的 `.cargo/config.toml` 中替换默认源：

```toml
[source.crates-io]
replace-with = "pavonis"

[source.pavonis]
registry = "sparse+https://cargo.example.com/index/"
```

## Go Modules

**仅用于公开模块的下载与 `sum.golang.org` 校验查询**，私有模块仍需配置 `GOPRIVATE` 由客户端自行拉取。

在站点中通过 `static_files` 为 `/sumdb/sum.golang.org/supported` 返回空的 `200` 响应，并将 `/sumdb/sum.golang.org` 映射到 `https://sum.golang.org`，即可让 Go 客户端自动通过同一个 `GOPROXY` 完成校验查询，无需改动 `GOSUMDB`：

```yaml
sites:
  - id: goproxy
    host: goproxy.example.com
    mode: http
    static_files:
      - path: /sumdb/sum.golang.org/supported
        content: ''
    settings:
      destination: https://proxy.golang.org
      redirect_action: follow_all
      mappings:
        - path: /sumdb/sum.golang.org
          destination: https://sum.golang.org
```

客户端配置：

```bash
go env -w GOPROXY=https://goproxy.example.com
```

## Gradle

**仅支持公开依赖、插件与 Gradle Wrapper 发行包的只读下载**。这三类资源位于不同上游，客户端也需要分别指定对应地址。

Gradle Plugin Portal 与 Wrapper 发行包下载均会返回跨域重定向，设置 `redirect_action: follow_all` 由 Pavonis 代为跟随，避免客户端直连外部对象存储：

```yaml
sites:
  - id: gradle
    host: gradle.example.com
    mode: http
    settings:
      redirect_action: follow_all
      mappings:
        - path: /central
          destination: https://repo.maven.apache.org/maven2
        - path: /portal
          destination: https://plugins.gradle.org/m2
        - path: /dist
          destination: https://services.gradle.org/distributions
```

在项目的 `settings.gradle` 或 `settings.gradle.kts` 中配置插件与依赖仓库：

```groovy
pluginManagement {
    repositories {
        maven { url = uri("https://gradle.example.com/portal/") }
        maven { url = uri("https://gradle.example.com/central/") }
    }
}

dependencyResolutionManagement {
    repositories {
        maven { url = uri("https://gradle.example.com/central/") }
    }
}
```

如需一并代理 Gradle Wrapper 发行包，修改 `gradle/wrapper/gradle-wrapper.properties` 中的 `distributionUrl`：

```properties
distributionUrl=https\://gradle.example.com/dist/gradle-9.2.1-bin.zip
```

## Homebrew

**仅用于安装 `homebrew/core` 的预编译 bottle**。从源码构建、安装 Cask 或拉取第三方 tap 仍会连接各自的上游地址。

Homebrew 安装预编译包涉及两条链路：`formulae.brew.sh/api` 提供签名公式元数据，GHCR 提供 bottle 制品。可以在同一域名下用 `path_prefix: /api` 的 `http` 站点代理元数据接口，其余路径交给限定了 `homebrew/core` 白名单的 `container_registry_single` 站点处理：

```yaml
sites:
  - id: brew-api
    host: brew.example.com
    mode: http
    path_prefix: /api
    settings:
      destination: https://formulae.brew.sh/api
      redirect_action: follow_all

  - id: brew-bottles
    host: brew.example.com
    mode: container_registry_single
    self_url: https://brew.example.com
    settings:
      upstream_v2_url: https://ghcr.io/v2
      upstream_auth_realm_url: https://ghcr.io/token
      repos_whitelist:
        - 'homebrew/core'
```

在客户端设置以下环境变量后即可安装软件包。其中 `QQ==` 是 Homebrew 访问 GHCR 公开仓库使用的固定匿名令牌，原样填写即可：

```bash
export HOMEBREW_API_DOMAIN=https://brew.example.com/api
export HOMEBREW_ARTIFACT_DOMAIN=https://brew.example.com
export HOMEBREW_DOCKER_REGISTRY_TOKEN=QQ==

brew install jq
```

## Maven Central

**仅支持只读代理 Maven Central 仓库**。构件发布不会经过代理，项目额外声明的其他仓库如需代理可通过 `mappings` 另行添加路径映射。

```yaml
sites:
  - id: maven
    host: maven.example.com
    mode: http
    settings:
      destination: https://repo.maven.apache.org/maven2
      redirect_action: follow_all
```

在 `~/.m2/settings.xml` 中将 `central` 指向本站点：

```xml
<settings>
  <mirrors>
    <mirror>
      <id>pavonis-central</id>
      <url>https://maven.example.com/</url>
      <mirrorOf>central</mirrorOf>
    </mirror>
  </mirrors>
</settings>
```

## RubyGems

**仅用于安装公开 gem 包**，不支持发布等需要认证的写操作。

```yaml
sites:
  - id: rubygems
    host: rubygems.example.com
    mode: http
    settings:
      destination: https://rubygems.org
      redirect_action: follow_all
```

使用 `gem` 安装：

```bash
gem install rake --clear-sources --source https://rubygems.example.com
```

使用 `Bundler` 时，修改 `Gemfile` 中的 `source`：

```ruby
source "https://rubygems.example.com"
```
