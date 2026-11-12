---
title: Outbound IP Pool
order: 1
---

# Outbound IP Pool

By default Pavonis uses the system outbound IP for all upstream requests. When you hold a large IPv6 address block (e.g. a `/48` subnet from HE Tunnel Broker), you can enable the **IP pool** to let Pavonis select an outbound IP from the pool according to a strategy, bypassing per-IP rate limits or access blocks imposed by upstream platforms.

The `random` strategy picks an outbound IP at random for each request, spreading requests across as many IPs as possible. The `ip_hash` strategy keeps the same client on the same outbound IP, which suits scenarios where the upstream needs to recognise the "same user" (e.g. APIs requiring session affinity). The global default strategy can be overridden per site.

```text
random: a random IP per request
  client A ─┬─ req 1 ──► 2001:db8::1a
            └─ req 2 ──► 2001:db8::7f
  client B ─── req 1 ──► 2001:db8::05

ip_hash: fixed IP per client
  client A ─┬─ req 1 ──► 2001:db8::1a
            └─ req 2 ──► 2001:db8::1a
  client B ─── req 1 ──► 2001:db8::33
```

See [`IpPoolConfig`](../config/request#ippoolconfig-ip-pool) and the site-level [`ip_pool_strategy`](../config/sites) for configuration.
