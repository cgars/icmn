# ADR 0001: PostgreSQL durability and transactional outbox

Status: Accepted (Phase 1)  
Date: 2026-09-10

## ELI5

Our notebook must remember its pages after it closes. PostgreSQL is that notebook. When we change a card, we write both the change and a note for the delivery helper on the same page: either both are saved or neither is. If the helper falls asleep, it can try the note again. A saved diary is not magically tamper-proof; a database operator can still alter it until later controls add independently verifiable checkpoints.

## Decision

Use PostgreSQL 17 as the durable adapter behind the application-owned `identity.Store` interface while retaining the in-memory adapter. Apply ordered, embedded SQL migrations transactionally at process startup. Use `database/sql` with pgx v5's standard-library driver: pgx supplies a maintained PostgreSQL protocol implementation and error codes needed to map uniqueness violations without string parsing.

Generate opaque entity, assertion, audit, and message IDs in the application. Store all instants as `timestamptz`, JSON assertion values as `jsonb`, and typed reference uniqueness as a database primary key over source system, object type, and source key.

Every successful mutation appends an audit row and an outbox row in its database transaction. Workers lease pending outbox rows with `FOR UPDATE SKIP LOCKED`; expired leases are recoverable and delivery is at-least-once. Consumers must deduplicate by message ID. Audit rows are append-only by application convention, not tamper-evident.

Idempotency keys identify one exact command payload. An exact replay returns the stored response; reusing a key for different input returns a typed conflict. Failed transactions retain neither business data nor idempotency/audit/outbox rows.

## Alternatives

- A hand-written PostgreSQL protocol was rejected as high-risk and unrelated to ICMN's purpose.
- An external migration framework was deferred: the initial forward-only ordered runner is small, tested against PostgreSQL, and avoids another dependency. Rollback guidance is still required before production claims.
- Publishing directly after commit was rejected because a crash can lose the notification.
- A broker was rejected for this phase because it adds infrastructure without improving the atomic database boundary.

## Consequences and verification

PostgreSQL is required for durable operation; no automatic fallback occurs when `DATABASE_URL` is configured incorrectly. Integration tests must run against a real server in CI and cover clean migration, reopen durability, concurrency, idempotency, ordering, and audit/outbox atomicity. Local memory mode remains convenient but is not durable or production-ready.
