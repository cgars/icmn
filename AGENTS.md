# Codex working agreement

This file is the operational entry point for coding agents working in ICMN.

## Mission

Build an identity-centric MDM system that minimizes ambiguity about *what* is referenced without forcing all domains to agree on *what it means*.

## Invariants

- Never place mutable business attributes on the core identity record.
- Never treat an external system identifier as the conserved identity.
- Every external reference names its source system, object type, and source key.
- Every semantic assertion names its domain, predicate, value, validity interval, and provenance.
- Conflicting domain assertions are valid data; do not resolve them by last-write-wins.
- Merges and splits are explicit, audited, and reversible.
- Matching produces evidence-bearing proposals. It must not silently merge identities.
- Keep the domain package independent of HTTP, databases, and vendors.

## Delivery order

1. Read `README.md`, `docs/principles.md`, `docs/architecture.md`, and `docs/roadmap.md`.
2. Pick one vertical slice from the current roadmap milestone.
3. State the invariant and acceptance criteria in the issue or pull request.
4. Add or change domain tests first.
5. Implement the smallest end-to-end slice.
6. Run `make check` and update documentation affected by the change.

## Engineering rules

- Use the Go standard library unless a dependency has a clear, recorded benefit.
- Keep packages cohesive and interfaces consumer-owned.
- Prefer explicit types over unstructured maps in the domain layer.
- Pass `context.Context` through application and persistence boundaries.
- Return typed errors that transports can map without parsing strings.
- Generate identifiers inside the application; do not expose database IDs.
- Store timestamps in UTC and serialize them as RFC 3339.
- Do not log assertion values or source payloads by default.
- Maintain backwards compatibility for released API versions.

## Required checks

```bash
make check
```

This runs formatting verification, vetting, and all tests. Add focused tests for each behavior and race tests for concurrent stores or resolvers.

## Architecture and security documentation

- Treat `docs/architecture/icmn-architecture.drawio` as the editable source for the primary architecture diagram. Keep its SVG and PNG exports synchronized in the same pull request.
- Update the diagram when a component, ownership boundary, trust boundary, persistence mechanism, connector capability, or material data flow changes.
- Read `docs/threat-model.md` before changing matching, merge/split behavior, authorization, tenancy, connectors, sensitive data handling, telemetry, backup, or deployment boundaries.
- Update the threat register, assumptions, security invariants, or release gates whenever a change introduces a threat, changes inherent/residual risk, or implements a listed control.
- Every material pull request must state: **architecture impact**, **threat-model impact**, and **diagram impact**. “None” requires a short justification.
- A security-sensitive feature is incomplete if its documentation and verification requirement remain stale, even when its functional tests pass.

## Commit scope

Keep commits reviewable and avoid mixing architectural refactors with features. Do not rewrite the paper merely to match an implementation shortcut; record deliberate changes to the conceptual model in an ADR.
