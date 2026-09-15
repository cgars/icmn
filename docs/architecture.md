# Architecture

ICMN begins as a modular monolith in Go. Clear internal boundaries preserve the option to separate workloads later without paying distributed-systems costs on day one.

## ELI5

ICMN keeps a map connecting cards about the same person or company. The cards can stay with their owners. Separate parts look after the map, each owner's statements, and the diary of decisions. A helper may fetch a card only when it has permission; knowing its address is not permission to open it.

This describes the target architecture. Phase 1 provides PostgreSQL identity/reference/assertion persistence, transactional audit/outbox records, and the retained in-memory adapter; matching, governed decisions, connectors, and the steward UI remain planned. See [the ELI5 guide](eli5.md) for examples of each concept.

## Core model

| Concept | Purpose | Ownership |
|---|---|---|
| Entity | Conserved, opaque identity | ICMN |
| External reference | Address of a source representation | Source connector/domain |
| Assertion | Contextual predicate and value | Domain |
| Provenance | Origin and production method | Producer |
| Match proposal | Evidence that identities may coincide | Resolver |
| Decision event | Governed merge, reject, or split | Steward/policy |

## Boundaries

1. **Identity registry** creates and resolves stable identities.
2. **Reference registry** guarantees that a typed source key refers to at most one active identity.
3. **Assertion ledger** accepts coexisting, temporal domain statements.
4. **Resolution pipeline** normalizes evidence and creates match proposals.
5. **Decision ledger** records merge and split decisions without erasing history.
6. **Connector boundary** dereferences external data only when authorized.

The initial in-memory adapter proves behavior. Persistence should be implemented behind consumer-owned interfaces, beginning with PostgreSQL. Events may initially use an outbox table; a broker is optional infrastructure, not part of the domain model.

## Dependency direction

`cmd/icmn` composes adapters. `internal/httpapi` depends on the application-facing `identity.Store` interface. The `identity` package depends only on the standard library. Database and connector adapters must depend inward on domain contracts.

## API policy

- Version public routes under `/v1`.
- Use opaque ICMN IDs in paths.
- Treat writes as commands with explicit conflict responses.
- Add idempotency keys before production ingestion.
- Never expose connector credentials or raw protected source payloads.

The hand-maintained OpenAPI 3.1 contract at [`api/openapi.json`](../api/openapi.json)
is the executable description of the current public boundary. Black-box transport
tests exercise the exported handler, while a contract drift test checks that every
route and method passed through the HTTP transport's single registration function
remains represented in the document.

### API compatibility

- Additive routes, optional fields, and response variants may be introduced when
  existing clients can continue to interpret the response safely.
- Released fields and their meanings are not silently removed or repurposed.
- A breaking transport or semantic change requires a new API version.
- Domain evolution belongs in versioned assertions and schemas; it must not mutate
  the semantics of the conserved identity or turn a contextual assertion into a
  universal attribute.

## Security baseline

Authentication and tenant isolation are required before multi-user deployment. Authorization must consider action, entity kind, domain, assertion predicate, and source system. Audit records must be append-only and redactable only through a separately recorded privacy operation.

The maintained [threat assessment](threat-model.md) defines the trust boundaries, threat register, security invariants, verification programme, and release gates. A feature is not complete merely because its happy path works: changes to matching, decisions, connectors, tenancy, or sensitive data flows must implement or explicitly track the corresponding controls.

## Architecture decisions to record next

- identity merge/split event model
- PostgreSQL schema and migration tool
- authorization policy engine boundary
- canonical JSON representation and assertion value limits
- connector execution and secret isolation

## Phase 1 durable registry

The implemented service can now compose either the retained in-memory adapter or a PostgreSQL adapter through the consumer-owned identity store contract. When `DATABASE_URL` is present, startup applies versioned migrations and fails rather than silently falling back. Identity, reference, assertion, audit, idempotency, and outbox writes share PostgreSQL transactions. Outbox delivery is recoverable and at-least-once; append-only application behavior is **not** tamper evidence.

### ELI5

Closing the service no longer erases the notebook when PostgreSQL is selected. Each change and its delivery note are saved together. The helper may deliver the same note twice after a crash, so receivers still check the note ID. The notebook has no unforgeable seal yet.
