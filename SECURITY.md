# Security

## Reporting

Please report security issues privately to the maintainer instead of opening a public issue. If no private advisory channel is available yet, email the maintainer listed on the GitHub profile.

## Data Handling

The default backend is `local`. It does not call a remote image provider.

The `openai`, `auto`, and `hybrid` backends can send source-derived visual packet content and scaffold HTML to OpenAI. Use these only for content that is acceptable to process remotely.

`--offline` disables remote source fetching and remote image generation.

Generated output files use private file permissions by default because bundles can contain source-derived content.

## Source Safety

Repo scans skip common secret files, generated directories, symlinks, oversized files, and binary-looking files. Direct local inputs that look like secrets, credentials, or private keys are rejected.

This is a local CLI for user-supplied inputs. Do not expose it as a network service without adding private-network URL guards, request authentication, and input allowlists.
