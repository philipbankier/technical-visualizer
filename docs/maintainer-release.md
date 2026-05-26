# Maintainer Release Notes

This file is for maintainers preparing v0.1.

## Before Tagging

- Confirm the feature branch has passed GitHub Actions.
- Confirm temporary API keys used for smoke tests have been deleted or rotated.
- Confirm no generated bundle contains secrets or absolute local paths.
- Confirm repo metadata is set on GitHub.
- Confirm `docs/release-readiness.md` matches the public release state.

## Manual Release

```bash
script/lint
script/test
script/smoke
GOTOOLCHAIN=go1.26.3 script/security
go test -cover ./...
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds darwin, linux, and windows assets and publishes a GitHub release from the tag.
