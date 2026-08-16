---
title: Documentation
order: 0
---

# Pavonis Documentation

Pavonis is a multi-purpose HTTP reverse proxy written in Go. A single YAML config file is all you need to run multiple proxy sites of different types simultaneously.

## Navigation

### 📖 Guide

Learn the fundamentals and get up and running quickly.

- [Getting Started](./guide/getting-started) — Installation, minimal config, and launching
- [Core Concepts](./guide/concepts) — Sites, modes, path prefixes, self_url explained

### ✨ Features

An overview of Pavonis's core capabilities.

- [Feature Overview](./features/) — Site modes, rate limiting, IP pool and more at a glance

### 📦 Site Modes

Detailed settings, parameters and examples for each `mode`.

- [GitHub Asset Acceleration](./sites/gh-proxy) — `gh_proxy` mode
- [Container Registry Proxy (Single)](./sites/container-registry-single) — `container_registry_single` mode
- [Container Registry Proxy (Any)](./sites/container-registry-any) — `container_registry_any` mode
- [General HTTP Reverse Proxy](./sites/http-proxy) — `http` mode
- [PyPI Mirror](./sites/pypi) — `pypi` mode
- [HuggingFace Download Proxy](./sites/hugging-face) — `hugging_face` mode
- [Speed Test](./sites/speed-test) — `speed_test` mode

### ⚙️ Config Reference

Complete documentation for every configuration field.

- [Config Overview](./config/) — Full config file example and top-level fields
- [ServerConfig](./config/server)
- [RequestConfig](./config/request)
- [ResponseConfig](./config/response)
- [ResourceLimitConfig](./config/resource-limit)
- [SiteConfig](./config/sites)
- [ContainerRegistryAuthConfig](./config/auth)
- [DiagnosticsConfig](./config/diagnostics)
