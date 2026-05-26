package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const codexBackendName = "codex"

type CodexConfig struct {
	BinaryPath string
}

type CodexBackend struct {
	binaryPath string
}

func NewCodexBackend(config CodexConfig) *CodexBackend {
	binaryPath := config.BinaryPath
	if binaryPath == "" {
		binaryPath = codexBackendName
	}
	return &CodexBackend{binaryPath: binaryPath}
}

func (b *CodexBackend) Name() string {
	return codexBackendName
}

func (b *CodexBackend) Available(ctx context.Context) Capability {
	binaryPath, err := resolveBinary(b.binaryPath)
	if err != nil {
		return Capability{
			Available: false,
			Name:      b.Name(),
			Reason:    err.Error(),
			Remote:    false,
		}
	}

	opCtx, cancel := operationContext(ctx, defaultOperationTimeout)
	defer cancel()

	// #nosec G204 -- this only probes the user-configured Codex CLI binary with a fixed --version argument.
	output, err := exec.CommandContext(opCtx, binaryPath, "--version").CombinedOutput()
	if err != nil {
		return Capability{
			Available: false,
			Name:      b.Name(),
			Reason:    fmt.Sprintf("%s --version failed: %v: %s", binaryPath, err, strings.TrimSpace(string(output))),
			Remote:    false,
		}
	}

	reason := strings.TrimSpace(string(output))
	if reason == "" {
		reason = binaryPath + " --version succeeded"
	}
	return Capability{
		Available: true,
		Name:      b.Name(),
		Reason:    reason,
		Remote:    false,
	}
}

func (b *CodexBackend) Generate(context.Context, ImageRequest) error {
	return errors.New("codex is available only as an agent workflow in v0.1; use --backend openai for direct API image generation or run Codex against the generated visual-packet.json and scaffold.html")
}

func resolveBinary(binaryPath string) (string, error) {
	if strings.ContainsRune(binaryPath, filepath.Separator) {
		if _, err := os.Stat(binaryPath); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("codex binary not found at %s", binaryPath)
			}
			return "", err
		}
		return binaryPath, nil
	}

	resolved, err := exec.LookPath(binaryPath)
	if err != nil {
		return "", fmt.Errorf("codex binary not found: %w", err)
	}
	return resolved, nil
}
