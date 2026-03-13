# CDS Documentation

## Concepts

| Document | Description |
|---|---|
| [Profiles](concepts/profiles.md) | What a profile is, temporary vs permanent, lifecycle, cookies |
| [Profile Schema](concepts/profile-schema.md) | Attribute types, mutability, merge strategies, core schema |
| [Unification Rules](concepts/unification-rules.md) | Rule structure, priority, how matching works |
| [How Unification Works](concepts/how-unification-works.md) | The full merge pipeline — userId match, rule evaluation, merge cases, data merging |

## Guides

| Document | Description |
|---|---|
| [IS Sync](guides/is-sync.md) | Identity Server event integration — user lifecycle and session events |
| [Schema Sync](guides/schema-sync.md) | Keeping CDS schema aligned with IS claim changes |
| [Extending Queue Providers](guides/extending-queue-providers.md) | Adding a new message queue provider (Kafka, RabbitMQ, SQS, etc.) |

## Issues / RFCs

| Document | Description |
|---|---|
| [Sessions as First-Class Profile Attribute](issues/rfc-sessions-as-first-class-profile-attribute.md) | Proposal to separate session stitching from profile unification references |
| [Configure Heartbeat Send/Receive](issues/configure-heartbeat-send-receive.md) | ActiveMQ heartbeat configuration |
