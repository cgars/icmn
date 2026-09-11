# ICMN — ELI5

Imagine a book connecting cards about people or companies. Each card has an owner who can say different things about the same subject. These comparisons explain the design; they are not promises that every feature exists today.

## Identity, references, and assertions

**Identity:** who are we talking about? **Reference:** where does a card about them live? **Assertion:** what does someone say about them?

Sales and Finance can describe the same company differently. "A great customer" and "Has unpaid bills" can both be useful statements. Connecting their cards does not make either statement a universal answer.

Phase 1 creates identities and attaches typed references and assertions either in memory or in a durable PostgreSQL notebook. It does not yet provide matching, merge/split, authorization, or the full production platform.

## Matching and confidence — planned

Two cards have similar names and addresses. That is a reason to look closer, not proof they describe the same company. A matching engine suggests; an authorized decision determines what happens next.

A score of 0.9 is not automatically "a 90% chance." Explain whether it is a similarity score or a calibrated probability, which evidence supports it, and what might contradict it.

## Merge and split — planned

A merge says, "We decided these cards describe the same company." Their original statements and the diary of the decision must remain traceable. A split corrects the connections when that decision was wrong.

Undoing a connection cannot make someone forget information already shared. The lifecycle ADR and implementation must define exactly what can be restored and how affected consumers are informed.

## Semantic contracts — planned

Before Sales sends Finance a "customer," they agree what that word means for this job. Is it the company paying the bill or the office receiving the parcel? They need an agreement for that exchange, not one definition for every job.

Contracts must make the purpose, versions, interpretations, and mapping explicit.

## Authorization — planned protections

Having a key to read two drawers does not mean you may join their cards or show their connection to somebody else. Knowing who you are and deciding what you may do are separate checks.

The seed is not ready for shared use with real data. See the [release gates](threat-model.md#release-gates).

## Time and provenance

"When was this supposed to be true? When did we learn it? Who told us?"

A statement received today may describe last week. Keep those times separate. The seed stores validity, recording time, and producer information; a supplied source name alone does not prove who sent it. Authenticated provenance and historical queries need further implementation.

## Architecture

ICMN keeps the map; source systems can keep their cards. A reference is an address, not a copy and not permission to fetch anything.

The [architecture diagram](architecture.md) describes the target. The current service has a durable PostgreSQL registry and transactional delivery notes; connectors, matching, governed decisions, and the steward interface are still planned.

## Steward actions — future UI guidance

Before a consequential action, explain what will change, what remains, who may be affected, and what undo can and cannot do.

Illustrative merge confirmation, to adapt to the implemented lifecycle:

> You are declaring that these records describe the same company. Their references will be connected. Both domains' statements will be preserved.

This is design guidance, not current UI behavior. The final confirmation must also describe actual downstream effects and reversal limits.

## Architectural decisions

An ADR should explain the choice in ELI5 terms before the technical reasoning. For example, a future persistence decision might say: "Our notebook must remember its pages after we close it. We also need to ensure two people cannot assign the same card to different subjects at the same time."

Then record the alternatives, trade-offs, and verification. A helpful comparison does not replace the decision.

## Risks and readiness

The [threat catalogue](threat-model.md#threat-register) explains each risk in its ELI5 column. Its controls are requirements, not a claim that protections are already installed. Release gates say what must be verified before a capability is safe to offer in the stated deployment.
