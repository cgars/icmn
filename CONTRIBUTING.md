# Contributing

Thank you for helping build ICMN.

Before opening a large change, create an issue describing the user problem, the domain invariant involved, and the smallest useful vertical slice. Architectural changes should include a short ADR under `docs/adr/`.

## ELI5

Explain your change as if to a curious five-year-old: what will happen, why it helps, and what could go wrong. Keep the precise technical explanation too. Follow the [ELI5 working agreement](AGENTS.md#eli5-explanations) and update existing explanations when behavior changes.

ADRs should include an ELI5 explanation of the decision and what it means in practice. User-facing confirmations should explain consequences and limits to reversal using the behavior actually implemented.

## Development

```bash
make check
make run
```

Pull requests should:

- include tests for changed behavior;
- preserve the invariants in `AGENTS.md`;
- update API or architecture documentation when relevant;
- avoid unrelated formatting or refactoring;
- contain no credentials, private datasets, or personal source records.

Use clear commit messages in the imperative mood. By submitting a contribution, you agree that it is licensed under the repository's MIT License.
