---
title: Supported Ecosystems
order: 4
---

# Supported Ecosystems

Beyond platforms with a dedicated `mode`, many package managers and registries can also be proxied by Pavonis using general-purpose modes such as [`http`](../sites/http). The table below lists upstream ecosystems tested to work with Pavonis and the modes they use:

| Ecosystem / Site | Mode | Description |
|------------------|------|-------------|
| [Conda](#conda) | [`http`](../sites/http) | Proxy `conda-forge` channel indexes and packages, compatible with `conda`, `mamba`, `micromamba` |
| [crates.io (Cargo)](#crates-io-cargo) | [`http`](../sites/http) | Proxy the Rust sparse index and `.crate` archive downloads |
| [GitHub](../sites/github) | [`github`](../sites/github) | Proxy Release assets, source archives, Raw files, Gist downloads, and `git clone/fetch/push` |
| [Go Modules](#go-modules) | [`http`](../sites/http) | Proxy `proxy.golang.org` module downloads and `sum.golang.org` checksum queries |
| [Gradle](#gradle) | [`http`](../sites/http) | Proxy Maven Central, Gradle Plugin Portal, and Gradle Wrapper distributions |
| [Homebrew](#homebrew) | [`http`](../sites/http) + [`container_registry_single`](../sites/container-registry-single) | Proxy signed formula metadata API and prebuilt `homebrew/core` bottles |
| [Hugging Face](../sites/hugging-face) | [`hugging_face`](../sites/hugging-face) | Proxy model and dataset file downloads, compatible with `hf` CLI, `transformers`, `datasets` |
| [Maven Central](#maven-central) | [`http`](../sites/http) | Proxy Maven Central POMs, dependencies, and build plugins |
| [npm](../sites/npm) | [`npm`](../sites/npm) | Proxy read-only npm registry metadata and tarballs, compatible with `npm`, `pnpm`, `yarn` |
| [OCI Container Registries](../sites/container-registry-single) | [`container_registry_single`](../sites/container-registry-single) / [`container_registry_any`](../sites/container-registry-any) | Proxy Docker Hub, GHCR, NVCR, and other container registries, compatible with `docker`, `containerd`, `crictl` |
| [PyPI](../sites/pypi) | [`pypi`](../sites/pypi) | Proxy the Python package index and file downloads, compatible with `pip`, `uv`, `Poetry` |
| [RubyGems](#rubygems) | [`http`](../sites/http) | Proxy Compact Index and `.gem` downloads, compatible with `gem` and `Bundler` |

For public deployments, serve Pavonis over HTTPS as described in [HTTPS and Reverse Proxy](./https). If you prefer not to use a separate domain per ecosystem, you can combine multiple upstreams under one domain with site `path_prefix` or `http` `mappings`.

## Conda

**Only channels that use relative paths in their repodata are supported**, such as `conda-forge`. If a channel returns absolute external download URLs in its index, clients still connect to those URLs directly. Because `conda-forge` redirects package downloads to object storage, set `redirect_action: follow_all` so Pavonis follows the redirects:

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

`conda`, `mamba`, and `micromamba` use the same flags to specify the channel:

```bash
micromamba create -n myenv --override-channels -c https://conda.example.com/conda-forge bzip2
```

Or configure it in `~/.condarc`:

```yaml
channels:
  - https://conda.example.com/conda-forge
```

## crates.io (Cargo)

**Only sparse index queries and `.crate` downloads are supported**, not `crates.io` web API operations such as publishing, yanking, or `cargo search`.

Cargo reads the `dl` field from `config.json` at the root of the index to construct `.crate` download URLs. Because upstream serves that file as `application/octet-stream`, configure `body_rewrites` to match both `application/json` and `application/octet-stream` and use `json_paths` to rewrite `dl` to this site's `/files` path:

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

Replace the default source in `~/.cargo/config.toml` or the project's `.cargo/config.toml`:

```toml
[source.crates-io]
replace-with = "pavonis"

[source.pavonis]
registry = "sparse+https://cargo.example.com/index/"
```

## Go Modules

**Only public module downloads and `sum.golang.org` checksum queries are proxied**. Private modules still require `GOPRIVATE` so the Go client fetches them directly.

Serving an empty `200` response at `/sumdb/sum.golang.org/supported` via `static_files` and mapping `/sumdb/sum.golang.org` to `https://sum.golang.org` lets Go route checksum verification through the same `GOPROXY` automatically, without changing `GOSUMDB`:

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

Client configuration:

```bash
go env -w GOPROXY=https://goproxy.example.com
```

## Gradle

**Only read-only downloads of public dependencies, plugins, and Gradle Wrapper distributions are supported**. Because these three come from separate upstreams, each must be configured on the client as well.

Both the Gradle Plugin Portal and Wrapper distribution downloads return cross-origin redirects, so set `redirect_action: follow_all` to have Pavonis follow them instead of exposing external storage hosts to the client:

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

Configure plugin and dependency repositories in the project's `settings.gradle` or `settings.gradle.kts`:

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

To proxy the Gradle Wrapper distribution as well, update `distributionUrl` in `gradle/wrapper/gradle-wrapper.properties`:

```properties
distributionUrl=https\://gradle.example.com/dist/gradle-9.2.1-bin.zip
```

## Homebrew

**Only prebuilt `homebrew/core` bottles are proxied**. Building from source, installing casks, and fetching third-party taps still connect to their respective upstreams.

Installing prebuilt bottles involves two upstreams: `formulae.brew.sh/api` serves signed formula metadata, and GHCR serves bottle blobs. Under one domain, route `/api` to an `http` site for the metadata API and let a `container_registry_single` site restricted to `homebrew/core` handle the remaining paths:

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

Export the following environment variables on the client before running `brew`. `QQ==` is Homebrew's fixed anonymous token for public GHCR access and should be used as-is:

```bash
export HOMEBREW_API_DOMAIN=https://brew.example.com/api
export HOMEBREW_ARTIFACT_DOMAIN=https://brew.example.com
export HOMEBREW_DOCKER_REGISTRY_TOKEN=QQ==

brew install jq
```

## Maven Central

**Only read-only access to Maven Central is proxied**. Artifact deployment is not supported, and other repositories declared by a project must be mapped separately via `mappings` if needed.

```yaml
sites:
  - id: maven
    host: maven.example.com
    mode: http
    settings:
      destination: https://repo.maven.apache.org/maven2
      redirect_action: follow_all
```

Point `central` to this site in `~/.m2/settings.xml`:

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

**Only installing public gems is supported**, without authenticated write operations such as publishing.

```yaml
sites:
  - id: rubygems
    host: rubygems.example.com
    mode: http
    settings:
      destination: https://rubygems.org
      redirect_action: follow_all
```

Install with `gem`:

```bash
gem install rake --clear-sources --source https://rubygems.example.com
```

With `Bundler`, set `source` in your `Gemfile`:

```ruby
source "https://rubygems.example.com"
```
