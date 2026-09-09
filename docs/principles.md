# Product principles

## Identity is conserved

An ICMN identity answers only: **what are we talking about?** It is stable, opaque, globally addressable, and independent of any source representation. Identity resolution may improve over time, so merge and split decisions form audited events rather than destructive rewrites.

## Meaning is negotiated

Meaning answers: **what does this entity mean here, for this purpose, at this time?** A semantic assertion therefore belongs to a named domain and carries provenance and temporal scope. Finance and Sales may make incompatible but individually valid assertions about the same identity.

## Data may remain elsewhere

ICMN is not automatically a data lake or replication hub. An external reference can point to a record through a stable URI or connector-specific key. Fetching source data is a separate, policy-controlled capability. This reduces copies, respects source ownership, and makes freshness explicit.

## Governance is a protocol

Governance establishes the rules under which identities, mappings, assertions, and match decisions are accepted. It does not manufacture universal semantics. Glossaries and canonical schemas are versioned agreements between communities, not timeless truth.

## Explainability over magic

A match proposal contains evidence, scores, algorithm/version information, and a decision state. Users must be able to understand why identities were proposed as equivalent and reverse an incorrect decision.
