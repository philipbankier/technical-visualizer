# Contributing

This repo is a Go CLI. Keep changes small, source-backed, and covered by tests.

## Setup

```bash
go version
go test ./...
```

Use the scripts in `script/` for the normal gates:

```bash
script/lint
script/test
script/smoke
script/security
```

## Pull Requests

- Keep defaults privacy-preserving.
- Do not introduce remote calls without a flag or an explicit backend.
- Do not add generated bundles, local caches, API keys, or screenshots to commits.
- Add tests for source gathering, prompt construction, or manifest behavior when you change those paths.
- Run `script/lint`, `script/test`, and `script/smoke` before opening a PR.

## Release Checks

Before tagging a release, run:

```bash
script/lint
script/test
script/smoke
script/security
go build -trimpath -o /tmp/visualize ./cmd/visualize
```
