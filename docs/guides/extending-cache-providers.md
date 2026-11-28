---
title: Extending the Cache — Adding a New Provider
date: 2026-11-27
---

# Extending the Cache — Adding a New Provider

This guide explains how to plug a shared cache store, such as Redis, Valkey or Memcached, into
Customer Data Directory (CDD).

## Overview

CDD caches data it reads on most requests: the org ID of each org handle, the unification rules
of each organization, and the CORS origins of each organization. By default, each instance keeps
these caches in its own memory. A write clears the entry at once on the instance that made it,
and other instances serve their copy until it expires.

A shared store lets every instance read the same entries, so a write is seen by all of them at
once. CDD ships no shared store; the cache layer in `internal/system/cache` only lets you add
one.

A provider supplies a `cache.Store`:

| Method | Description |
|---|---|
| `Get(ctx, namespace, key)` | Return the value and `true`, or `false` when there is none or it has expired. |
| `Set(ctx, namespace, key, value, ttl)` | Store the value until the TTL passes. |
| `Delete(ctx, namespace, key)` | Remove the key. A missing key is not an error. |
| `Clear(ctx, namespace)` | Remove every key of the namespace. |
| `Close()` | Release connections at shutdown. |

- Each cache has a namespace, such as `organization.refs_by_handle`. Keep the keys of one
  namespace apart from those of another, for example with a `namespace:key` prefix.
- Values arrive as JSON bytes. Store them as they are.
- A store must be safe for concurrent use.
- An error is never fatal. CDD logs it and reads the database instead, so a store that is down
  makes CDD slower, not unavailable.
- `Clear` runs only when unification rules change, which is rare. A store without a cheap way to
  remove a namespace can keep a generation number per namespace and put it in each key.

Some caches always stay in memory, whatever the provider: the value frequencies read while
scoring matches, because a network round trip there costs more than the read it saves.

## Step-by-step guide

### 1. Create the provider package

```
internal/system/cache/
└── myprovider/
    └── myprovider.go
```

### 2. Implement `cache.Store` and register it

```go
package myprovider

import (
	"github.com/wso2/identity-customer-data-service/internal/system/cache"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

func init() {
	cache.RegisterProvider("myprovider", func(cfg config.CacheConfig) (cache.Store, error) {
		return newStore(cfg.Properties["addr"], cfg.Properties["password"])
	})
}
```

The provider reads its settings from `cache.properties`. Return an error when a setting is
missing or the store cannot be reached: the server stops at startup rather than running without
the store it was configured with.

### 3. Import the package from the server

The provider's `init()` only runs if the package is imported. Add a blank import in
`internal/system/server/server.go`, next to the one for the ActiveMQ queue provider:

```go
_ "github.com/wso2/identity-customer-data-service/internal/system/cache/myprovider"
```

### 4. Select it in `deployment.yaml`

```yaml
cache:
  type: "myprovider"
  properties:
    addr: "cache.example.internal:6379"
    password: "${CACHE_PASSWORD}"
```

An unknown `type` stops the server at startup.

## Securing the store

CDD trusts what it reads from the store. The store holds the org ID of each org handle, which
decides the organization a request reads and writes, and the CORS origins of each organization.
Anyone who can write to the store can therefore send the requests of one organization to the data
of another.

- Make the store reachable only by CDD, on a private network.
- Require authentication, and use TLS between CDD and the store.
- Give each deployment its own store, or its own key prefix, so that two deployments never share
  entries.
- Take the password from an environment variable (`${CACHE_PASSWORD}` above) rather than writing it in `deployment.yaml`.

## Adding a cache

Create a cache with a type, a name of its own and a TTL:

```go
var refs = cache.New[model.Ref]("organization.refs_by_handle", 5*time.Minute)
```

- With a shared store, values are stored as JSON, so only exported fields are kept.
- Pass `cache.Local()` to keep a cache in memory on every deployment.
