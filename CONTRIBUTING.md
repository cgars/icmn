# Contributing

Thank you for helping build ICMN.

Before opening a large change, create an issue describing the user problem, the domain invariant involved, and the smallest useful vertical slice. Architectural changes should include a short ADR under `docs/adr/`.

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
