# Agent Workflows

Agents are useful around `technical-visualizer` because the CLI writes source-backed intermediate files that are easy to inspect.

## Local Review Workflow

```bash
visualize --backend local --renderer html --out visualize-output https://github.com/example/service
codex -C . "Review visualize-output/manifest.json and visualize-output/scaffold.html. Identify missing evidence, weak claims, and any privacy risks."
```

Use this workflow when you want an agent to audit the bundle without asking the CLI to call a remote image provider.

## Codex Image Handoff Workflow

In supported Codex environments, Codex may expose a built-in image-generation tool. That is different from the standalone Go CLI calling OpenAI directly.

```bash
visualize --backend local --renderer html --handoff codex --quick --out visualize-output https://github.com/example/service
```

Use the printed POSIX shell command in an interactive Codex session. If quick mode is not used, open `visualize-output/handoff/codex-prompt.md` and paste it into Codex manually.

Use this path when you want to use Codex or ChatGPT subscription access through the agent environment. The agent should save the generated image back into the bundle as `final.png` or another clear filename.

## Content Pack Handoff Workflow

```bash
visualize --pack auto --backend local --renderer html --handoff codex --quick --out visualize-output https://github.com/example/service
```

Use the printed POSIX shell command in an interactive Codex session. If quick mode is not used, open `visualize-output/handoff/content-pack-codex-prompt.md` and use the target briefs under `visualize-output/pack/`.

The agent should save each generated target image to the `output_path` listed in `content-pack.json`, for example `pack/linkedin-dense/final.png`.

## Direct OpenAI API Image Workflow

```bash
OPENAI_API_KEY=... visualize --backend openai --renderer image --out visualize-output https://github.com/example/service
```

Use this path when you want the Go CLI itself to call the OpenAI Images API. This requires `OPENAI_API_KEY` and can incur OpenAI API usage.

## Boundaries

- `visualize --backend codex` is not supported in v0.1.
- A ChatGPT or Codex subscription is separate from API billing and does not cover `--backend openai`.
- Single-image agent outputs should be saved back into the bundle as `final.png`; content-pack outputs should use each target `output_path`.
- Do not hand private source to a remote agent or remote image tool unless that is acceptable for the project.
