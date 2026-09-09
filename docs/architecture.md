# Architecture

ICMN begins as a modular monolith in Go. Clear internal boundaries preserve the option to separate workloads later without paying distributed-systems costs on day one.

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
