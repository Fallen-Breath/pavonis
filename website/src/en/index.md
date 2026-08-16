---
layout: home

hero:
  name: Pavonis
  text: Gateway to Digital Infinities
  tagline: A multi-purpose reverse proxy supporting container registries, GitHub assets, HuggingFace models, PyPI and more — with built-in IP pool and traffic rate limiting
  actions:
    - theme: brand
      text: Getting Started
      link: /docs/guide/getting-started
    - theme: alt
      text: Features
      link: /docs/features/
    - theme: alt
      text: Config Reference
      link: /docs/config/

features:
  - title: Container Registry Proxy
    details: Proxy Docker Hub, GHCR, NVCR and any OCI-compatible registry. Supports client authentication and repository allow/deny lists.
  - title: GitHub Asset Acceleration
    details: Proxy GitHub Releases, Raw files and Gists. Supports automatic URL rewriting in text content and response body size limits.
  - title: HuggingFace Download Proxy
    details: Accelerate HuggingFace model and dataset downloads. Compatible with huggingface-cli and auto-rewrites redirect URLs.
  - title: PyPI Mirror
    details: Proxy the PyPI package index and rewrite download links in HTML and JSON responses. Compatible with pip and friends.
  - title: General HTTP Reverse Proxy
    details: Route requests to different upstreams via path-prefix mappings. Supports multiple redirect handling strategies.
  - title: Outbound IP Pool
    details: Configure an IPv6 subnet pool as outbound IPs. Supports random selection and client-IP-based hash assignment.
---
