---
title: Site Modes
order: 3
---

# Site Modes

Each site specifies its proxy behaviour via the `mode` field. This section documents the purpose, configuration options and examples for every mode.

| Mode | Config String | Purpose |
|------|--------------|---------|
| [GitHub Asset Acceleration](./github) | `github` | Proxy GitHub Releases, Raw files, Gist downloads |
| [Container Registry Proxy (Single)](./container-registry-single) | `container_registry_single` | Proxy a specific OCI container registry |
| [Container Registry Proxy (Any)](./container-registry-any) | `container_registry_any` | Proxy any registry by writing the source registry in the image name |
| [General HTTP Reverse Proxy](./http) | `http` | Route requests to different upstreams by path prefix |
| [Any URL Proxy](./any-url) | `any_url` | Proxy an upstream URL embedded in the request path |
| [PyPI Mirror](./pypi) | `pypi` | Proxy the PyPI index and rewrite download links |
| [npm Mirror](./npm) | `npm` | Proxy an npm registry and rewrite download links in package metadata |
| [Hugging Face Download Proxy](./hugging-face) | `hugging_face` | Accelerate Hugging Face model and dataset downloads |
| [Speed Test](./speed-test) | `speed_test` | Provide upload/download speed test endpoints (experimental) |

Beyond the dedicated modes above, common upstreams such as Cargo, Conda, Go Modules, Gradle, Homebrew, Maven, and RubyGems can be proxied through general-purpose modes. See [Supported Ecosystems](../guide/ecosystems).
