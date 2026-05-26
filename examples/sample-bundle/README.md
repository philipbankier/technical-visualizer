# Sample Bundle

This sample shows a checked-in excerpt of a local `technical-visualizer` output bundle.

Regenerate and prune it from the repository root:

```bash
rm -rf examples/sample-bundle/output
go run ./cmd/visualize --backend local --renderer html --offline --out examples/sample-bundle/output examples/sample-bundle/input/notes.md
rm -f examples/sample-bundle/output/final.png
jq '(.output_files) |= map(select(.kind != "image"))' examples/sample-bundle/output/manifest.json > /tmp/technical-visualizer-sample-manifest.json
mv /tmp/technical-visualizer-sample-manifest.json examples/sample-bundle/output/manifest.json
```

The committed excerpt includes only `output/scaffold.html`, `output/visual-packet.json`, and `output/manifest.json`. Inspect `output/scaffold.html` first. `output/final.png` is intentionally not checked in because the local PNG is only a fallback preview in v0.1.
