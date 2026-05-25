package source

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type secretRedactionPattern struct {
	pattern     *regexp.Regexp
	replacement string
	reason      string
}

var secretRedactionPatterns = []secretRedactionPattern{
	{
		pattern:     regexp.MustCompile(`(?i)((?:"|')?authorization(?:"|')?\s*[:=]\s*(?:"|')?bearer\s+)([^"',\r\n]{12,})(["']?)`),
		replacement: "${1}[REDACTED]${3}",
		reason:      "authorization bearer token redacted",
	},
	{
		pattern:     regexp.MustCompile(`(?i)((?:"|')?(?:api[_-]?key|token|secret|password|passwd)(?:"|')?\s*[:=]\s*)(["'])([^"'\r\n]{8,})(["'])`),
		replacement: "${1}${2}[REDACTED]${4}",
		reason:      "quoted secret value redacted",
	},
	{
		pattern:     regexp.MustCompile(`(?i)((?:"|')?(?:api[_-]?key|token|secret|password|passwd|authorization)(?:"|')?\s*[:=]\s*)([A-Za-z0-9._/\-+=]{12,})`),
		replacement: "${1}[REDACTED]",
		reason:      "unquoted secret value redacted",
	},
}

func gatherLocalMarkdown(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if err := ctx.Err(); err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "%s", err.Error())}}
	}
	path := sourceTarget(spec)
	data, warning := readLocalBounded(path, opts)
	if warning != "" {
		return gatherResult{Warnings: []string{sourceWarning(spec, "%s", warning)}}
	}
	text, redactions := redactLikelySecrets(spec.ID, path, string(data))
	return gatherResult{
		Items: []model.EvidenceItem{{
			ID:       evidenceID(spec.ID, path),
			SourceID: spec.ID,
			Kind:     "markdown",
			Title:    filepath.Base(path),
			Text:     text,
			Path:     path,
			SHA256:   sha256Hex(data),
		}},
		Redactions: redactions,
	}
}

func gatherLocalJSONPacket(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if err := ctx.Err(); err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "%s", err.Error())}}
	}
	path := sourceTarget(spec)
	data, warning := readLocalBounded(path, opts)
	if warning != "" {
		return gatherResult{Warnings: []string{sourceWarning(spec, "%s", warning)}}
	}
	text, redactions := redactLikelySecrets(spec.ID, path, string(data))
	return gatherResult{
		Items: []model.EvidenceItem{{
			ID:       evidenceID(spec.ID, path),
			SourceID: spec.ID,
			Kind:     "visual_packet_json",
			Title:    filepath.Base(path),
			Text:     text,
			Path:     path,
			SHA256:   sha256Hex(data),
		}},
		Redactions: redactions,
	}
}

func gatherLocalPDF(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if err := ctx.Err(); err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "%s", err.Error())}}
	}
	path := sourceTarget(spec)
	info, err := os.Stat(path)
	if err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "PDF stat failed for %q: %v", path, err)}}
	}
	if info.Size() > opts.MaxBytesPerFile {
		return gatherResult{Warnings: []string{sourceWarning(spec, "PDF %q exceeds MaxBytesPerFile (%d > %d); skipped text extraction", path, info.Size(), opts.MaxBytesPerFile)}}
	}
	return gatherResult{Warnings: []string{sourceWarning(spec, "PDF text extraction is not implemented in v1 for %q; file size %d bytes was bounded and no text evidence was emitted", path, info.Size())}}
}

func gatherLocalRepo(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	root := sourceTarget(spec)
	result := gatherResult{}
	included := 0
	limitWarningRecorded := false

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "failed to inspect %q: %v", path, walkErr))
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "failed to make %q relative to %q: %v", path, root, err))
			return nil
		}
		rel = filepath.ToSlash(rel)

		if entry.Type()&fs.ModeSymlink != 0 {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "skipped symlink %q by safety default", rel))
			return nil
		}

		if entry.IsDir() {
			if shouldSkipRepoDir(entry.Name()) {
				result.Warnings = append(result.Warnings, sourceWarning(spec, "skipped directory %q by safety default", rel))
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipSecretPath(rel) {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "skipped secret-like file %q", rel))
			return nil
		}
		if included >= opts.MaxFiles {
			if !limitWarningRecorded {
				result.Warnings = append(result.Warnings, sourceWarning(spec, "stopped local scan after MaxFiles=%d", opts.MaxFiles))
				limitWarningRecorded = true
			}
			return filepath.SkipAll
		}
		if !shouldIncludeRepoFile(rel) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "failed to stat %q: %v", rel, err))
			return nil
		}
		if info.Size() > opts.MaxBytesPerFile {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "skipped %q because it exceeds MaxBytesPerFile (%d > %d)", rel, info.Size(), opts.MaxBytesPerFile))
			return nil
		}

		// #nosec G304,G122 -- WalkDir supplies this path from the caller-selected source root; symlinks are skipped above.
		data, err := os.ReadFile(path)
		if err != nil {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "failed to read %q: %v", rel, err))
			return nil
		}
		if looksBinary(data) {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "skipped binary-looking file %q", rel))
			return nil
		}

		text, redactions := redactLikelySecrets(spec.ID, rel, string(data))
		result.Redactions = append(result.Redactions, redactions...)
		result.Items = append(result.Items, model.EvidenceItem{
			ID:       evidenceID(spec.ID, rel),
			SourceID: spec.ID,
			Kind:     repoEvidenceKind(rel),
			Title:    rel,
			Text:     text,
			Path:     rel,
			SHA256:   sha256Hex(data),
		})
		included++
		return nil
	})
	if err != nil {
		result.Warnings = append(result.Warnings, sourceWarning(spec, "local scan failed for %q: %v", root, err))
	}

	return result
}

func readLocalBounded(path string, opts GatherOptions) ([]byte, string) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Sprintf("stat failed for %q: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Sprintf("%q is a symlink; skipped", path)
	}
	if info.IsDir() {
		return nil, fmt.Sprintf("%q is a directory", path)
	}
	if info.Size() > opts.MaxBytesPerFile {
		return nil, fmt.Sprintf("%q exceeds MaxBytesPerFile (%d > %d)", path, info.Size(), opts.MaxBytesPerFile)
	}
	// #nosec G304 -- direct local files are caller-selected inputs and secret-like paths are rejected above.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("read failed for %q: %v", path, err)
	}
	if looksBinary(data) {
		return nil, fmt.Sprintf("%q looks binary; skipped", path)
	}
	return data, ""
}

func shouldSkipRepoDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", "dist", "build":
		return true
	default:
		return false
	}
}

func shouldSkipSecretPath(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	base := slashpath.Base(normalized)
	if shouldSkipSecretFile(base) {
		return true
	}
	if normalized == ".aws/credentials" || strings.HasSuffix(normalized, "/.aws/credentials") {
		return true
	}
	if normalized == ".kube/config" || strings.HasSuffix(normalized, "/.kube/config") {
		return true
	}
	if strings.Contains(normalized, "/config/secrets.") || strings.HasPrefix(normalized, "config/secrets.") {
		return true
	}
	return false
}

func shouldSkipSecretFile(name string) bool {
	lower := strings.ToLower(name)
	if lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return true
	}
	switch lower {
	case ".npmrc", ".pypirc", ".netrc", "credentials", "credentials.json", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519":
		return true
	}
	switch filepath.Ext(lower) {
	case ".pem", ".key", ".p12", ".pfx":
		return true
	}
	return strings.HasPrefix(lower, "secrets.")
}

func shouldIncludeRepoFile(rel string) bool {
	normalized := filepath.ToSlash(strings.ToLower(rel))
	base := filepath.Base(normalized)
	ext := filepath.Ext(base)

	if strings.HasPrefix(base, "readme") {
		return true
	}
	if ext == ".md" || ext == ".markdown" {
		return true
	}

	switch base {
	case "go.mod", "go.sum", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
		"tsconfig.json", "jsconfig.json", "pyproject.toml", "cargo.toml", "composer.json",
		"gemfile", "requirements.txt", "dockerfile", "makefile", "visual-packet.json":
		return true
	}

	if strings.HasPrefix(normalized, "cmd/") || base == "main.go" {
		return isTextLikeExtension(ext)
	}

	if strings.Contains(normalized, "route") || strings.Contains(normalized, "api") || strings.Contains(normalized, "schema") || strings.Contains(normalized, "config") {
		return isTextLikeExtension(ext)
	}

	return false
}

func isTextLikeExtension(ext string) bool {
	switch ext {
	case "", ".go", ".js", ".jsx", ".ts", ".tsx", ".json", ".yaml", ".yml", ".toml", ".graphql", ".gql", ".proto", ".rs", ".py", ".java", ".kt", ".rb", ".php", ".cs", ".xml", ".txt", ".md", ".markdown":
		return true
	default:
		return false
	}
}

func repoEvidenceKind(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".md" || ext == ".markdown" || strings.HasPrefix(base, "readme") {
		return "markdown"
	}
	switch base {
	case "visual-packet.json":
		return "visual_packet_json"
	case "go.mod", "go.sum", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
		"tsconfig.json", "jsconfig.json", "pyproject.toml", "cargo.toml", "composer.json",
		"gemfile", "requirements.txt":
		return "metadata"
	default:
		return "source_file"
	}
}

func looksBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	control := 0
	for _, b := range data {
		if b < 0x09 || (b > 0x0d && b < 0x20) {
			control++
		}
	}
	return control > len(data)/10
}

func redactLikelySecrets(sourceID string, locator string, text string) (string, []model.Redaction) {
	redacted := text
	var redactions []model.Redaction
	for _, redactionPattern := range secretRedactionPatterns {
		if !redactionPattern.pattern.MatchString(redacted) {
			continue
		}
		redacted = redactionPattern.pattern.ReplaceAllString(redacted, redactionPattern.replacement)
		redactions = append(redactions, model.Redaction{
			SourceID: sourceID,
			Path:     locator,
			Reason:   redactionPattern.reason,
		})
	}
	return redacted, redactions
}
