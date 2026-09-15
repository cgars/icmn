# ICMN

**Identity Conserved. Meaning Negotiated.**

ICMN is an open-source, identity-centric master data platform. It establishes durable enterprise identities while allowing each domain to maintain its own contextual assertions and terminology.

The project intentionally separates three things that conventional MDM often collapses:

- **Identity** — the stable referent shared across systems.
- **References** — links to representations held in source systems; source data need not be copied into ICMN.
- **Meaning** — versioned, attributable assertions made by domains about an identity.

![ICMN architecture: a conserved identity plane connected to negotiated domain meaning through provenance and governed references](docs/architecture/icmn-architecture.svg)

## ELI5

Imagine Sales and Finance each have a card about the same company. Sales writes, "We like working with them." Finance writes, "They have not paid." ICMN connects the cards so we know who they mean, while keeping who said what visible. Connecting the cards does not make either statement everybody's answer.

The picture above shows the intended architecture, including parts still to be built. The current Phase 1 service durably stores identities, references, assertions, audit events, and delivery notes when PostgreSQL is configured; an in-memory adapter remains available. Read [ICMN explained like you are five](docs/eli5.md) for the concepts and their limits.

## Status

This repository is at the durable-registry stage. It remains a local, unauthenticated prototype rather than a production-ready or multi-tenant service.

## Quick start

Requires Go 1.27 or newer.

```bash
go test ./...
go run ./cmd/icmn
```

The server listens on `:8080` by default. Set `ICMN_ADDR` to override it.

```bash
curl -s -X POST http://localhost:8080/v1/entities \
  -H 'Content-Type: application/json' \
  -d '{"kind":"organization"}'
```

The response contains the conserved ICMN identity. Domain assertions and external references can then be attached without turning either into the identity itself.

## Repository map

```text
cmd/icmn/            service entry point
internal/identity/   domain model and identity operations
internal/httpapi/    HTTP transport
docs/                architecture, principles, and roadmap
paper/               source of the accompanying paper
.github/             CI and contribution automation
```

Start with [AGENTS.md](AGENTS.md) when using Codex, [docs/architecture.md](docs/architecture.md) when making design decisions, and [docs/threat-model.md](docs/threat-model.md) before changing a trust boundary or security-sensitive workflow.

## Scope of the first usable release

- durable identities and aliases
- externally held source records represented by typed references
- domain-owned, temporal assertions
- provenance and an append-only change history
- deterministic matching proposals, never silent merges
- API-first operation with pluggable persistence

See [docs/roadmap.md](docs/roadmap.md) for staged delivery and explicit non-goals.

## Principles

1. An identity is not a golden record.
2. Source systems remain authoritative for their representations.
3. Assertions always have a domain, provenance, and time.
4. Conflicting assertions may coexist.
5. Automated matching proposes; governed decisions merge or split.
6. Every consequential change must be explainable and reversible.

## Paper

The conceptual argument lives in [`paper/icmn.tex`](paper/icmn.tex). Build it with:

```bash
make paper
```

## Contributing and security

Contributions are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md). Please report vulnerabilities according to [SECURITY.md](SECURITY.md).

## License

ICMN is licensed under the [MIT License](LICENSE).

## Durable local evaluation (Phase 1)

This local prototype is single-tenant, unauthenticated, and only suitable for fictional data. Docker Compose publishes both development ports on loopback, not the network. From a clean checkout, start PostgreSQL and ICMN with one command:

```bash
docker compose up --build
```

Then use `http://127.0.0.1:8080`. The named volume preserves data across container restarts; `docker compose down -v` deliberately deletes it. Do not use the checked-in local password outside this fictional local setup. Running `go run ./cmd/icmn` without `DATABASE_URL` retains the non-durable in-memory adapter.
