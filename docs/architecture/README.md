# Architecture diagram assets

The editable source of the primary architecture diagram is `icmn-architecture.drawio`. Its text-based SVG export is committed and used by the main README. The binary PNG is generated from the committed Draw.io source in GitHub Actions and uploaded as a workflow artifact tied to that run and commit; it is not committed.

When a component, trust boundary, ownership boundary, or material data flow changes:

1. update the Draw.io source;
2. export a new SVG with embedded fonts disabled;
3. verify the SVG visually, including its labels, arrows, boundaries, and implemented/planned distinctions;
4. update `docs/architecture.md` and `docs/threat-model.md` when the change affects their claims.

With the Draw.io desktop CLI installed:

```bash
make diagrams
```

CI renders the downloadable PNG with a digest-pinned Draw.io container and checks that the 2× bitmap is a readable 3200×2000 image. The container's headless renderer gets 120 seconds to start and export instead of its 10-second default, which is too short on a fresh CI runner. To reproduce that artifact locally, install Docker and run:

```bash
make diagram-png verify-diagram-png
```

After a workflow completes, open its summary in GitHub Actions and download the `icmn-architecture-png-<commit SHA>` artifact. Visually inspect the PNG before relying on it in a publication. The automated size check catches missing, corrupt, or unexpectedly low-resolution output, but it cannot judge clipped text or misleading arrows.

Do not hand-edit generated exports. If the SVG requires a GitHub-specific accessibility correction, reproduce that correction in the Draw.io source where possible and document any unavoidable difference here.
