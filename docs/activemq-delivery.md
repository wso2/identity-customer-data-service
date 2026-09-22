# ActiveMQ delivery and failure handling

CDS uses STOMP 1.2 point-to-point destinations named `/queue/...`. Successful
enqueue waits for a broker receipt and requests persistent storage. The broker
must itself have persistence enabled. An enqueue error can have an uncertain
outcome (the broker may have accepted the message before the connection failed);
the single reconnect/send retry can therefore produce duplicates.

Consumers use individual acknowledgements and prefetch one message. The worker
returns an error to the queue, and only a nil result is acknowledged as successful.
An error is handled according to the following policy:

| Result | Handling |
| --- | --- |
| Success, including a profile deleted before processing | Acknowledge |
| Error explicitly marked `queue.Retryable` | Up to three total processing attempts |
| Unmarked error or malformed payload | Transfer to the source destination plus `.DLQ` |
| Retry allowance exhausted | Transfer to the same dead-letter destination |
| Worker draining or shutdown cancellation | Leave unacknowledged for redelivery |

Retry copies carry `cds-processing-attempt`. The second and third attempts wait
two and four seconds respectively. These conservative fixed limits bound
application retry amplification and reuse the existing two-second connection
backoff; they are not operation deadlines or workload-derived latency targets.
The counter survives a committed retry transfer and process restart. A crash or
lost acknowledgement can repeat the current attempt, so this is not a global
exactly-three-executions guarantee. FIFO processing order is not guaranteed:
retries are appended to the queue.

Only profile reads before merge writes are marked retryable. Profile write
failures can leave partially committed state, so they go to the DLQ for inspection
instead of automatic application retry. Schema synchronization re-reads the
current schema and performs a transactional upsert; its failures may be retried.
Invalid payloads and unknown failures are terminal by default.

## Dead-letter contract and broker prerequisites

For source `/queue/cds-profile-unification`, provision and monitor
`/queue/cds-profile-unification.DLQ`; do the same for the configured schema queue.
Grant the CDS principal read/write access to the source and write access to its
DLQ (and destination creation permissions if using automatic creation). Grant
DLQ read access only to the appropriate operators. Use durable broker storage.
The integration suite verifies the actual destination against ActiveMQ Classic
5.18.3, the repository's existing test version.

The replacement SEND and source ACK are in one STOMP transaction. CDS waits for
the COMMIT receipt before reporting the transfer. If transfer or settlement
fails, CDS reconnects and lets the broker resolve/redeliver pending work.
It never acknowledges the source separately after a failed DLQ publish.
This application-owned destination does not depend on the broker's default
`ActiveMQ.DLQ`, NACK policy, Java client redelivery policy, or a scheduler plugin.

DLQ messages preserve the original body and content type, plus the original
message ID, source destination, attempt and a failure category. They do not copy
arbitrary broker headers or raw error text. Queue outcome logs contain category
and attempt, not payloads. DLQ bodies still contain customer data: apply the
same access controls and retention policy as the source. Alert on DLQ growth
and repeated settlement/reconnect failures. Inspect and repair partial profile
state before manually replaying a dead-letter message.

## Connection recovery and shutdown

Long-lived consumers continue reconnecting during broker outages, using
exponential delays from two seconds up to sixty seconds. Publishing permits
only one reconnect and one send retry. Reconnect ownership is acquired with
a timeout; publishers cannot wait behind an infinite consumer retry loop.
Failed subscriptions also back off instead of repeatedly reading a closed channel.

TCP/TLS connection establishment, the STOMP handshake, socket writes and receipt
waits are bounded. A retry/DLQ transaction has a ten-second transfer limit, enforced
by closing its socket if its receipt never arrives. Closing the queue interrupts
a pending handshake and retry waits, prevents new connections from being installed,
and retains PR 3's shared shutdown deadline for disconnect.

The worker drain counts business processing, not the final broker acknowledgement.
If shutdown wins that last race, completed work may be redelivered. Normal
connection loss can also repeat completed work. This is at-least-once delivery,
not exactly-once database processing. PR 5 must establish atomic and idempotent
profile unification; this PR does not fix concurrent lost updates or partial merges.

The in-memory provider accepts the same error-returning callback and logs failure,
but remains non-durable and does not retry or persist failed work.
Custom queue providers must migrate their `Start` callback to return `error`.

## Verification

`make mq-integration-test` includes isolated queue tests using a real ActiveMQ
container, alongside the existing PostgreSQL/worker integration tests.
To run just queue tests against an isolated local broker with admin/admin credentials:

```sh
CDS_TEST_ACTIVEMQ_ADDR=127.0.0.1:61623 go test -race ./test/queue_integration/...
```

The queue tests cover success, transient recovery, exhausted retries, terminal
errors, malformed messages, transactional DLQ transfer, deferred work and
unacknowledged in-flight work and recovery after connection loss. Protocol tests cover acknowledgement ordering,
missing commit receipts and shutdown during a handshake.

Protocol references: [STOMP 1.2 transactions](https://stomp.github.io/stomp-specification-1.2.html#BEGIN),
[ActiveMQ STOMP headers](https://activemq.apache.org/components/classic/documentation/stomp),
and [ActiveMQ redelivery/DLQ policies](https://activemq.apache.org/components/classic/documentation/message-redelivery-and-dlq-handling).
