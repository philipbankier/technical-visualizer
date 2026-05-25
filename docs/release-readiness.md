# Release Readiness

Date: 2026-05-25

Scope: `visualizer/` as the standalone repository root for `github.com/philipbankier/technical-visualizer`.

Do not publish the parent workspace as this tool's GitHub repo. The parent workspace contains unrelated app code and exploration folders.

## Resolved For OSS Prep

- Public module path set to `github.com/philipbankier/technical-visualizer`.
- CLI defaults to the local backend so source-derived content is not uploaded when `OPENAI_API_KEY` happens to be present.
- OpenAI image size changed to `1536x1024`, a supported landscape size for GPT image generation.
- `visualize --version` added.
- Direct secret-like local file inputs are rejected.
- Repo scans skip path-aware secret files, symlinks, generated directories, oversized files, and binary-looking files.
- Output-facing bundles sanitize absolute local source paths.
- Generated bundles and cache directories use private permissions.
- OpenAI image generation uses a five minute timeout after live testing showed 30 seconds was too short for high quality generation.
- OSS files added: `LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, `.gitignore`, `AGENTS.md`.
- Release scripts and GitHub Actions workflows added.

## Release Gates

Run from this directory:

```bash
script/lint
script/test
script/smoke
script/security
```

If local Go is behind the latest patch release, run with an explicit patched toolchain:

```bash
GOTOOLCHAIN=go1.26.3 script/security
```

## Manual Release

```bash
script/lint
script/test
script/smoke
script/security
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds darwin, linux, and windows assets and publishes a GitHub release from the tag.

## Remaining Decisions

- Confirm the final GitHub repo name. Current code assumes `github.com/philipbankier/technical-visualizer`.
- Rotate or delete any temporary API key used for live release smoke testing.
- Decide whether private-network URL blocking should be built before anyone wraps the CLI in a hosted service.

## Live OpenAI Smoke

Completed on 2026-05-25 with a temporary API key supplied through an interactive shell environment only.

- `visualize doctor` reported OpenAI available.
- Default backend with `OPENAI_API_KEY` present still wrote `backend.name=local`.
- Explicit `--backend openai --renderer image` wrote a valid `1536x1024` PNG and manifest with `backend.model=gpt-image-2`.
- `--backend auto --renderer image` selected OpenAI and wrote a valid `1536x1024` PNG.
- Generated bundle scans found no API key or absolute local path leaks.
