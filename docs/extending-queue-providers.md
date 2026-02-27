---
title: Extending the Message Queue — Adding a New Provider
date: 2026-02-27
---

# 🔌 Extending the Message Queue — Adding a New Provider

This guide explains how to add a new message queue provider to the CDS
pluggable queue architecture (e.g. RabbitMQ, Kafka, Amazon SQS, etc.).

---

## Overview

The queue layer is built around two interfaces defined in
`internal/system/queue/queue.go`:

| Interface | Purpose |
|---|---|
| `ProfileUnificationQueue` | Enqueue and process profile-unification events |
| `SchemaSyncQueue` | Enqueue and process schema-synchronisation events |

Providers register themselves at startup via the factory's provider registry
(see `internal/system/queue/factory.go`), following the same pattern as Go's
`database/sql` driver model. No modification to the factory or any other
existing file is required when adding a new provider.

---

## Step-by-step guide

### 1. Create the provider package

Create a new sub-package under `internal/system/queue/`:

```
internal/system/queue/
├── activemq/        ← existing ActiveMQ provider
├── inmemory/        ← built-in default
└── myprovider/      ← your new provider
    └── myprovider.go
```

### 2. Implement the two interfaces

Your package must provide types that satisfy `queue.ProfileUnificationQueue`
and `queue.SchemaSyncQueue`:

```go
package myprovider

import (
    profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
    schemaModel  "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
    "github.com/wso2/identity-customer-data-service/internal/system/config"
    "github.com/wso2/identity-customer-data-service/internal/system/log"
    "github.com/wso2/identity-customer-data-service/internal/system/queue"
)

// ProfileQueue implements queue.ProfileUnificationQueue.
type ProfileQueue struct { /* broker connection fields */ }

func (q *ProfileQueue) Enqueue(profile profileModel.Profile) bool {
    // publish profile to your broker
    return true
}

func (q *ProfileQueue) Start(handler func(profileModel.Profile)) error {
    // subscribe and forward messages to handler in a goroutine
    go func() { /* consume loop */ }()
    return nil
}

// SchemaSyncQueue implements queue.SchemaSyncQueue.
type SchemaSyncQueue struct { /* broker connection fields */ }

func (q *SchemaSyncQueue) Enqueue(sync schemaModel.ProfileSchemaSync) bool {
    // publish sync job to your broker
    return true
}

func (q *SchemaSyncQueue) Start(handler func(schemaModel.ProfileSchemaSync)) error {
    // subscribe and forward messages to handler in a goroutine
    go func() { /* consume loop */ }()
    return nil
}
```

### 3. Register the provider via `init()`

Add an `init()` function in your package that registers both queues with the
factory. The name you register with is the value users will set in
`message_queue.type` in `deployment.yaml`.

```go
func init() {
    queue.RegisterProfileQueueProvider("myprovider",
        func(cfg config.ExternalBrokerConfig) (queue.ProfileUnificationQueue, error) {
            return newProfileQueue(cfg.Addr, cfg.Username, cfg.Password, cfg.ProfileQueueName)
        },
    )
    queue.RegisterSchemaSyncQueueProvider("myprovider",
        func(cfg config.ExternalBrokerConfig) (queue.SchemaSyncQueue, error) {
            return newSchemaSyncQueue(cfg.Addr, cfg.Username, cfg.Password, cfg.SchemaSyncQueueName)
        },
    )
}
```

The `config.ExternalBrokerConfig` struct provides the common broker settings
(`Addr`, `Username`, `Password`, `ProfileQueueName`, `SchemaSyncQueueName`).
If your broker requires additional settings not covered by
`ExternalBrokerConfig`, read them from environment variables inside your
constructor.

### 4. Activate the provider in `cmd/server/main.go`

The provider's `init()` only runs if the package is imported. Add a blank
import alongside the existing ActiveMQ one:

```go
import (
    // ...
    _ "github.com/wso2/identity-customer-data-service/internal/system/queue/activemq"   // existing
    _ "github.com/wso2/identity-customer-data-service/internal/system/queue/myprovider" // your new provider
)
```

> **Tip:** Only import the providers your deployment actually needs. Unused
> providers add binary size but are otherwise harmless.

### 5. Configure `deployment.yaml`

Set `message_queue.type` to the name you registered and fill in the broker
coordinates:

```yaml
message_queue:
  type: "myprovider"
  broker:
    addr: "mybroker:5672"
    username: "${BROKER_USERNAME}"
    password: "${BROKER_PASSWORD}"
    profile_queue_name: "/queue/cds-profile-unification"
    schema_sync_queue_name: "/queue/cds-schema-sync"
```

### 6. (Optional) Add an integration test

Mirror the test in `test/activemq_integration/` to spin up your broker via
testcontainers and run an end-to-end profile unification scenario:

```
test/
└── myprovider_integration/
    ├── main_test.go
    └── profile_unification_myprovider_test.go
```

---

## Summary

| Step | What to do |
|---|---|
| 1 | Create `internal/system/queue/myprovider/` |
| 2 | Implement `ProfileUnificationQueue` and `SchemaSyncQueue` |
| 3 | Register via `init()` using `queue.Register*QueueProvider` |
| 4 | Blank-import in `cmd/server/main.go` |
| 5 | Set `message_queue.type` in `deployment.yaml` |
| 6 | (Optional) Add integration tests |

No changes to the factory or any other existing file are needed.
