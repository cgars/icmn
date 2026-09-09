# Architecture diagram assets

The editable source of the primary architecture diagram is `icmn-architecture.drawio`. The SVG and PNG files are rendered derivatives used by GitHub and other publications.

When a component, trust boundary, ownership boundary, or material data flow changes:

1. update the Draw.io source;
2. export a new SVG with embedded fonts disabled;
3. export a 2× PNG on a light background;
4. verify both renderings visually;
5. update `docs/architecture.md` and `docs/threat-model.md` when the change affects their claims.

With the Draw.io desktop CLI installed:

```bash
make diagrams
```

Do not hand-edit the generated PNG. If the SVG requires a GitHub-specific accessibility correction, reproduce that correction in the Draw.io source where possible and document any unavoidable difference here.
