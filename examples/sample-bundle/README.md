# Sample Bundle

This sample shows the shape of a local `technical-visualizer` output bundle.

Regenerate it from the repository root:

```bash
go run ./cmd/visualize --backend local --renderer html --offline --out examples/sample-bundle/output examples/sample-bundle/input/notes.md
```

Inspect `output/scaffold.html` first. `output/final.png` is intentionally not checked in because the local PNG is only a fallback preview in v0.1.
