package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func ResolveAll(inputs []string) ([]model.SourceSpec, error) {
	specs := make([]model.SourceSpec, 0, len(inputs))
	for _, input := range inputs {
		spec, err := ResolveOne(input)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func ResolveOne(input string) (model.SourceSpec, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return model.SourceSpec{}, fmt.Errorf("empty source input")
	}

	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" {
		if u.Scheme != "http" && u.Scheme != "https" {
			return model.SourceSpec{}, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
		}
		return resolveURL(trimmed, u), nil
	}

	return resolvePath(trimmed)
}

func resolveURL(input string, u *url.URL) model.SourceSpec {
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.Path)
	kind := model.SourceDocsSite

	switch {
	case host == "github.com" && hasRepoPath(u.Path):
		kind = model.SourceGitHubRepo
	case strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".markdown"):
		kind = model.SourceMarkdown
	case strings.HasSuffix(path, ".pdf"):
		kind = model.SourcePDF
	}

	return model.SourceSpec{
		ID:       stableID(input),
		Kind:     kind,
		Input:    input,
		Resolved: input,
	}
}

func hasRepoPath(path string) bool {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	return len(segments) >= 2 && segments[0] != "" && segments[1] != ""
}

func resolvePath(input string) (model.SourceSpec, error) {
	abs, err := filepath.Abs(input)
	if err != nil {
		return model.SourceSpec{}, err
	}
	if shouldSkipSecretPath(input) || shouldSkipSecretPath(abs) {
		return model.SourceSpec{}, fmt.Errorf("refusing secret-like source path %q", input)
	}
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".json":
		return sourceSpec(input, abs, model.SourceJSONPacket), nil
	case ".md", ".markdown":
		return sourceSpec(input, abs, model.SourceMarkdown), nil
	case ".pdf":
		return sourceSpec(input, abs, model.SourcePDF), nil
	}

	info, err := os.Stat(abs)
	if err != nil {
		return model.SourceSpec{}, err
	}

	kind := model.SourceLocalRepo
	if !info.IsDir() {
		kind = model.SourceMarkdown
	}

	return sourceSpec(input, abs, kind), nil
}

func sourceSpec(input string, resolved string, kind model.SourceKind) model.SourceSpec {
	return model.SourceSpec{
		ID:       stableID(resolved),
		Kind:     kind,
		Input:    input,
		Resolved: resolved,
	}
}

func stableID(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "src-" + hex.EncodeToString(sum[:])[:12]
}
