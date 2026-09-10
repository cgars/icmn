# Roadmap

The roadmap is ordered by risk reduction, not feature count.

## Milestone 0 — executable model (current)

- [x] Go module and HTTP service
- [x] conserved identity creation
- [x] typed external references
- [x] domain assertions with provenance
- [x] tests for reference uniqueness and semantic coexistence
- [x] API contract and integration test suite

## Milestone 1 — durable registry

- PostgreSQL persistence and migrations
- idempotent write commands
- append-only audit events
- pagination and query by external reference
- containerized local development
- OpenTelemetry traces and structured metrics

## Milestone 2 — resolution

- normalization profiles by entity kind
- deterministic rules and evidence model
- candidate generation
- scored match proposals
- steward accept/reject workflow
- reversible merge and split

## Milestone 3 — negotiated semantics

- assertion schemas and constraints by domain
- validity and recording time queries
- semantic agreement lifecycle and glossary links
- conflict views without forced winner selection

## Milestone 4 — federation

- connector SDK and capability model
- policy-controlled dereferencing
- change feeds and webhooks
- deployment hardening, tenancy, and fine-grained authorization

## Explicit non-goals for early releases

- universal canonical business model
- opaque machine-learning auto-merge
- copying complete source records by default
- bespoke workflow engine
- replacing data catalogs, IAM systems, or event brokers
