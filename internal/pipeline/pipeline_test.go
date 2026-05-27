package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/backend"
	"github.com/philipbankier/technical-visualizer/internal/model"
	"github.com/philipbankier/technical-visualizer/internal/pack"
)

func TestRunOfflineAutoUsesLocalBackendEvenWithAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nOffline mode must not upload source-derived content."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "auto",
		Renderer:  "hybrid",
		Offline:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local non-remote", manifest.Backend)
	}
	if !manifest.Audit.FallbackUsed {
		t.Fatalf("manifest audit fallback = false, want true for offline auto")
	}
	if manifest.Audit.WarningCount != len(manifest.Warnings) {
		t.Fatalf("manifest audit warning count = %d, want %d", manifest.Audit.WarningCount, len(manifest.Warnings))
	}
	if _, err := os.Stat(filepath.Join(outputDir, "final.png")); err != nil {
		t.Fatalf("Stat(final.png) error = %v", err)
	}
}

func TestRunFailsWhenAllSourcesProduceNoEvidence(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "missing.md")
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want no evidence error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no evidence") {
		t.Fatalf("Run() error = %q, want no evidence explanation", err)
	}
	if strings.Contains(err.Error(), root) {
		t.Fatalf("Run() error leaked temp root %q: %v", root, err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("manifest.json exists after no-evidence run, stat error = %v", statErr)
	}
}

func TestRunPDFOnlySucceedsWithPopplerTextEvidence(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "paper.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	useFakePDFToText(t, "pdftotext version 99.1.0", strings.Repeat("SkillOpt reports source-backed PDF extraction with readable architecture evaluation methods and limitations. ", 5), nil)
	outputDir := filepath.Join(root, "out")

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(manifest.SourceDiagnostics) != 1 {
		t.Fatalf("SourceDiagnostics = %#v, want one PDF diagnostic", manifest.SourceDiagnostics)
	}
	diagnostic := manifest.SourceDiagnostics[0]
	if diagnostic.SourceID != manifest.Sources[0].ID || diagnostic.Kind != "pdf" || diagnostic.Engine != "pdftotext" {
		t.Fatalf("SourceDiagnostics[0] = %#v, want pdftotext PDF diagnostic", diagnostic)
	}
	if diagnostic.Version != "pdftotext version 99.1.0" || diagnostic.PagesAttempted != 1 || diagnostic.PagesExtracted != 1 {
		t.Fatalf("SourceDiagnostics[0] = %#v, want version and page counts", diagnostic)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}
	if !strings.Contains(string(data), `"source_diagnostics"`) || !strings.Contains(string(data), `"pdftotext version 99.1.0"`) {
		t.Fatalf("manifest.json missing PDF diagnostics: %s", data)
	}
}

func TestRunPDFOnlyFailsWithInstallGuidanceWhenPopplerUnavailable(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "paper.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", t.TempDir())

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: filepath.Join(root, "out"),
		Backend:   "local",
		Renderer:  "html",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want no-evidence PDF failure")
	}
	errText := err.Error()
	for _, want := range []string{"no evidence", "install Poppler"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("Run() error = %q, want %q", errText, want)
		}
	}
	if strings.Contains(errText, root) || strings.Contains(errText, sourcePath) {
		t.Fatalf("Run() error leaked local path: %v", err)
	}
}

func TestRunPDFOnlyFailsWithLowEvidenceGuidance(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "paper.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	useFakePDFToText(t, "pdftotext version 99.1.0", "OCR", nil)

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: filepath.Join(root, "out"),
		Backend:   "local",
		Renderer:  "html",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want low-evidence PDF failure")
	}
	errText := err.Error()
	for _, want := range []string{"no evidence", "too little readable text"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("Run() error = %q, want %q", errText, want)
		}
	}
	if strings.Contains(errText, root) || strings.Contains(errText, sourcePath) {
		t.Fatalf("Run() error leaked local path: %v", err)
	}
}

func TestRunMixedMarkdownAndFailedPDFKeepsWarning(t *testing.T) {
	root := t.TempDir()
	markdownPath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(markdownPath, []byte("# System\n\nMarkdown evidence lets the run proceed."), 0o644); err != nil {
		t.Fatalf("WriteFile(markdown) error = %v", err)
	}
	pdfPath := filepath.Join(root, "paper.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(pdf) error = %v", err)
	}
	t.Setenv("PATH", t.TempDir())

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{markdownPath, pdfPath},
		OutputDir: filepath.Join(root, "out"),
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !hasWarningContaining(manifest.Warnings, "install Poppler") {
		t.Fatalf("warnings = %#v, want Poppler install guidance", manifest.Warnings)
	}
	if len(manifest.SourceDiagnostics) != 1 || len(manifest.SourceDiagnostics[0].Warnings) == 0 {
		t.Fatalf("SourceDiagnostics = %#v, want PDF warning diagnostic", manifest.SourceDiagnostics)
	}
}

func TestRunPDFOutputsDoNotLeakLocalPaths(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "private", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nfixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	useFakePDFToText(t, "pdftotext version 99.1.0", strings.Repeat("PDF source output should retain source-backed text without leaking private local file paths or temp directories. ", 5), nil)
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, name := range []string{"visual-packet.json", "manifest.json", "scaffold.html"} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		text := string(data)
		if strings.Contains(text, root) || strings.Contains(text, sourcePath) || strings.Contains(text, os.TempDir()) {
			t.Fatalf("%s leaked local path or temp dir:\n%s", name, data)
		}
	}
}

func TestRunNoEvidenceCleansStaleGeneratedBundleFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThis run writes bundle files."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	for _, name := range generatedBundleFiles {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Fatalf("Stat(%s) after initial run error = %v", name, err)
		}
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	_, err = Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want no evidence error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no evidence") {
		t.Fatalf("Run() error = %q, want no evidence explanation", err)
	}
	for _, name := range generatedBundleFiles {
		if _, statErr := os.Stat(filepath.Join(outputDir, name)); !os.IsNotExist(statErr) {
			t.Fatalf("%s exists after no-evidence rerun, stat error = %v", name, statErr)
		}
	}
}

func TestRunPackAutoWritesPackAndBriefs(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, rel := range []string{"content-pack.json", "pack/linkedin-dense/brief.md", "pack/social-teaser/brief.md", "pack/blog-og/brief.md"} {
		info, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", rel, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %o, want 0600", rel, info.Mode().Perm())
		}
	}
	if !hasOutputKind(manifest.OutputFiles, "content_pack") {
		t.Fatalf("manifest missing content_pack output: %#v", manifest.OutputFiles)
	}
	if countOutputKind(manifest.OutputFiles, "pack_brief") != 3 {
		t.Fatalf("manifest pack_brief outputs = %#v, want three", manifest.OutputFiles)
	}
	if hasOutputKind(manifest.OutputFiles, "pack_image") {
		t.Fatalf("local planned pack should not declare generated images: %#v", manifest.OutputFiles)
	}
	assertNextStepContains(t, manifest.NextSteps, "content-pack.json")
	assertNextStepContains(t, manifest.NextSteps, filepath.Join("pack"))

	briefData, err := os.ReadFile(filepath.Join(outputDir, "pack", "linkedin-dense", "brief.md"))
	if err != nil {
		t.Fatalf("ReadFile(linkedin brief) error = %v", err)
	}
	brief := string(briefData)
	for _, want := range []string{
		"Technical Map:",
		"Target intent",
		"technical deep-dive infographic",
		"Density",
		"high",
		"Aspect ratio",
		"16:9",
		"Required content",
		"Avoid",
		"Source references",
		"planned local output is not a final polished image",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief missing %q:\n%s", want, brief)
		}
	}
}

func TestRunPackCodexHandoffWritesPackPrompt(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
		Pack:      "auto",
		Quick:     true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	promptPath := filepath.Join(outputDir, "handoff", "content-pack-codex-prompt.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile(pack prompt) error = %v", err)
	}
	for _, want := range []string{
		"linkedin-dense",
		"social-teaser",
		"blog-og",
		"content-pack.json",
		"visual-packet.json",
		"scaffold.html",
		"pack/<target>/brief.md",
		"not `visualize --backend codex`",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("pack prompt missing %q:\n%s", want, data)
		}
	}
	if !hasOutputKind(manifest.OutputFiles, "handoff_pack_prompt") {
		t.Fatalf("manifest output files missing pack prompt: %#v", manifest.OutputFiles)
	}

	packData, err := os.ReadFile(filepath.Join(outputDir, "content-pack.json"))
	if err != nil {
		t.Fatalf("ReadFile(content-pack.json) error = %v", err)
	}
	var contentPack pack.ContentPack
	if err := json.Unmarshal(packData, &contentPack); err != nil {
		t.Fatalf("Unmarshal(content-pack.json) error = %v", err)
	}
	if contentPack.Status != pack.StatusHandoffReady {
		t.Fatalf("content pack status = %q, want %q", contentPack.Status, pack.StatusHandoffReady)
	}
	for _, target := range contentPack.Targets {
		if target.State != pack.StateHandoffReady {
			t.Fatalf("target %q state = %q, want %q", target.ID, target.State, pack.StateHandoffReady)
		}
	}
}

func TestRunCodexHandoffWithoutPackCleansStalePackPrompt(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThis run toggles pack handoff output."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	packPromptPath := filepath.Join(outputDir, "handoff", "content-pack-codex-prompt.md")
	if _, err := os.Stat(packPromptPath); err != nil {
		t.Fatalf("Stat(pack prompt) after initial run error = %v", err)
	}

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
		Pack:      "off",
	})
	if err != nil {
		t.Fatalf("rerun without pack error = %v", err)
	}
	if _, statErr := os.Stat(packPromptPath); !os.IsNotExist(statErr) {
		t.Fatalf("stale pack prompt exists after pack-disabled codex rerun, stat error = %v", statErr)
	}
	if !hasOutputKind(manifest.OutputFiles, "handoff_prompt") {
		t.Fatalf("manifest missing single-image handoff prompt after rerun: %#v", manifest.OutputFiles)
	}
	if hasOutputKind(manifest.OutputFiles, "handoff_pack_prompt") {
		t.Fatalf("manifest declares stale pack handoff prompt after rerun: %#v", manifest.OutputFiles)
	}
}

func TestRunRejectsUnsupportedPackBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Pack:      "carousel",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want unsupported pack failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unsupported pack") {
		t.Fatalf("Run() error = %q, want unsupported pack explanation", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "content-pack.json")); !os.IsNotExist(statErr) {
		t.Fatalf("content-pack.json exists after rejected run, stat error = %v", statErr)
	}
}

func TestRunNoEvidenceCleansStaleGeneratedPackFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThis run writes pack files."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	for _, rel := range []string{"content-pack.json", "pack/linkedin-dense/brief.md", "pack/social-teaser/brief.md", "pack/blog-og/brief.md"} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("Stat(%s) after initial run error = %v", rel, err)
		}
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	_, err = Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want no evidence error")
	}
	for _, rel := range []string{"content-pack.json", "pack/linkedin-dense/brief.md", "pack/social-teaser/brief.md", "pack/blog-og/brief.md"} {
		if _, statErr := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); !os.IsNotExist(statErr) {
			t.Fatalf("%s exists after no-evidence rerun, stat error = %v", rel, statErr)
		}
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "pack")); !os.IsNotExist(statErr) {
		t.Fatalf("pack dir exists after no-evidence cleanup, stat error = %v", statErr)
	}
}

func TestRunPackDisabledCleansGeneratedPackFilesAndPreservesCustomPackFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThis run writes and then disables pack files."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	customPath := filepath.Join(outputDir, "pack", "operator-note.md")
	if err := os.WriteFile(customPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("WriteFile(custom pack file) error = %v", err)
	}

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "off",
	})
	if err != nil {
		t.Fatalf("disabled Run() error = %v", err)
	}
	if hasOutputKind(manifest.OutputFiles, "content_pack") || hasOutputKind(manifest.OutputFiles, "pack_brief") {
		t.Fatalf("disabled pack run declared pack outputs: %#v", manifest.OutputFiles)
	}
	for _, rel := range []string{"content-pack.json", "pack/linkedin-dense/brief.md", "pack/social-teaser/brief.md", "pack/blog-og/brief.md"} {
		if _, statErr := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); !os.IsNotExist(statErr) {
			t.Fatalf("%s exists after disabled pack rerun, stat error = %v", rel, statErr)
		}
	}
	data, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile(custom pack file) error = %v", err)
	}
	if string(data) != "keep me" {
		t.Fatalf("custom pack file = %q, want preserved", data)
	}
}

func TestRunPackAutoRejectsSymlinkedTargetDirectory(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nSymlinked pack target dirs must not be followed."), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	outputDir := filepath.Join(root, "out")
	if err := os.MkdirAll(filepath.Join(outputDir, "pack"), 0o700); err != nil {
		t.Fatalf("MkdirAll(pack) error = %v", err)
	}
	outside := t.TempDir()
	outsideBrief := filepath.Join(outside, "brief.md")
	if err := os.WriteFile(outsideBrief, []byte("outside"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside brief) error = %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(outputDir, "pack", "linkedin-dense")); err != nil {
		t.Skipf("Symlink() unsupported: %v", err)
	}

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Run() error = %v, want symlink rejection", err)
	}
	data, err := os.ReadFile(outsideBrief)
	if err != nil {
		t.Fatalf("ReadFile(outside brief) error = %v", err)
	}
	if string(data) != "outside" {
		t.Fatalf("outside brief = %q, want unchanged", data)
	}
}

func TestRunPackDisabledRejectsSymlinkedTargetDirectoryBeforeCleanup(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nSymlinked pack target dirs must not be cleaned through."), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "auto",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	if err := os.RemoveAll(filepath.Join(outputDir, "pack", "linkedin-dense")); err != nil {
		t.Fatalf("RemoveAll(linkedin-dense) error = %v", err)
	}
	outside := t.TempDir()
	outsideBrief := filepath.Join(outside, "brief.md")
	if err := os.WriteFile(outsideBrief, []byte("outside"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside brief) error = %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(outputDir, "pack", "linkedin-dense")); err != nil {
		t.Skipf("Symlink() unsupported: %v", err)
	}

	_, err = Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Pack:      "off",
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Run() error = %v, want symlink rejection", err)
	}
	data, err := os.ReadFile(outsideBrief)
	if err != nil {
		t.Fatalf("ReadFile(outside brief) error = %v", err)
	}
	if string(data) != "outside" {
		t.Fatalf("outside brief = %q, want unchanged", data)
	}
}

func TestRunNoEvidenceCleansStaleHandoffFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThis run writes handoff files."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "handoff", "codex-prompt.md")); err != nil {
		t.Fatalf("Stat(codex-prompt.md) after initial run error = %v", err)
	}
	customPath := filepath.Join(outputDir, "handoff", "operator-note.md")
	if err := os.WriteFile(customPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("WriteFile(custom handoff file) error = %v", err)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	_, err = Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want no evidence error")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "handoff", "codex-prompt.md")); !os.IsNotExist(statErr) {
		t.Fatalf("generated prompt exists after no-evidence rerun, stat error = %v", statErr)
	}
	data, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile(custom handoff file) error = %v", err)
	}
	if string(data) != "keep me" {
		t.Fatalf("custom handoff file = %q, want preserved", data)
	}
}

func TestRunFailureAfterExistingBundleRestoresPreviousBundle(t *testing.T) {
	useOpenAIFake(t, fakeOpenAIBackend{generateErr: errors.New("forced OpenAI failure")})
	root := t.TempDir()
	oldSource := filepath.Join(root, "old.md")
	newSource := filepath.Join(root, "new.md")
	if err := os.WriteFile(oldSource, []byte("# Old Source\n\nPrevious bundle content."), 0o644); err != nil {
		t.Fatalf("WriteFile(old source) error = %v", err)
	}
	if err := os.WriteFile(newSource, []byte("# New Source\n\nThis failed run must not replace the prior bundle."), 0o644); err != nil {
		t.Fatalf("WriteFile(new source) error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{oldSource},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
	})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	beforeManifest, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest before) error = %v", err)
	}
	beforePrompt, err := os.ReadFile(filepath.Join(outputDir, "handoff", "codex-prompt.md"))
	if err != nil {
		t.Fatalf("ReadFile(prompt before) error = %v", err)
	}

	_, err = Run(context.Background(), Options{
		Sources:   []string{newSource},
		OutputDir: outputDir,
		Backend:   "openai",
		Renderer:  "image",
		Handoff:   "codex",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want forced OpenAI failure")
	}

	afterManifest, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest after) error = %v", err)
	}
	if string(afterManifest) != string(beforeManifest) {
		t.Fatalf("manifest changed after failed rerun\nbefore:\n%s\nafter:\n%s", beforeManifest, afterManifest)
	}
	afterPrompt, err := os.ReadFile(filepath.Join(outputDir, "handoff", "codex-prompt.md"))
	if err != nil {
		t.Fatalf("ReadFile(prompt after) error = %v", err)
	}
	if string(afterPrompt) != string(beforePrompt) {
		t.Fatalf("handoff prompt changed after failed rerun")
	}
}

func TestRunRejectsSymlinkedHandoffDirectoryBeforeRewrite(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nSymlinked handoff dirs must not be followed."), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	outputDir := filepath.Join(root, "out")
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(output) error = %v", err)
	}
	outside := t.TempDir()
	outsidePrompt := filepath.Join(outside, "codex-prompt.md")
	if err := os.WriteFile(outsidePrompt, []byte("outside"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside prompt) error = %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(outputDir, "handoff")); err != nil {
		t.Skipf("Symlink() unsupported: %v", err)
	}

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Run() error = %v, want symlink rejection", err)
	}
	data, err := os.ReadFile(outsidePrompt)
	if err != nil {
		t.Fatalf("ReadFile(outside prompt) error = %v", err)
	}
	if string(data) != "outside" {
		t.Fatalf("outside prompt = %q, want unchanged", data)
	}
}

func TestBackupGeneratedBundleFilesRejectsSymlinkedHandoffDirectory(t *testing.T) {
	outputDir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(outputDir, "handoff")); err != nil {
		t.Skipf("Symlink() unsupported: %v", err)
	}

	_, err := backupGeneratedBundleFiles(outputDir)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("backupGeneratedBundleFiles() error = %v, want symlink rejection", err)
	}
}

func TestRunAutoFallsBackWhenOpenAIGenerationFails(t *testing.T) {
	useOpenAIFake(t, fakeOpenAIBackend{generateErr: errors.New("forced OpenAI failure")})
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nFallback should preserve a local bundle."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "auto",
		Renderer:  "image",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local fallback", manifest.Backend)
	}
	if !manifest.Audit.RemoteImageAttempted {
		t.Fatalf("RemoteImageAttempted = false, want true")
	}
	if !manifest.Audit.FallbackUsed {
		t.Fatalf("FallbackUsed = false, want true")
	}
	if !hasWarningContaining(manifest.Warnings, "OpenAI generation failed") {
		t.Fatalf("warnings = %#v, want OpenAI fallback warning", manifest.Warnings)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "final.png")); err != nil {
		t.Fatalf("Stat(final.png) error = %v", err)
	}
}

func TestRunExplicitOpenAIFailsWhenGenerationFails(t *testing.T) {
	useOpenAIFake(t, fakeOpenAIBackend{generateErr: errors.New("forced OpenAI failure")})
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nExplicit OpenAI should fail fast."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "openai",
		Renderer:  "image",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want explicit OpenAI failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "openai") {
		t.Fatalf("Run() error = %q, want OpenAI explanation", err)
	}
}

func TestRunDefaultBackendStaysLocalWithAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nDefault mode must not upload source-derived content."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local non-remote", manifest.Backend)
	}
}

func TestBaselineAuditCopiesImageGenerationMetadata(t *testing.T) {
	imageResult := imageGenerationResult{
		BackendInfo:             model.BackendInfo{Name: "openai", Remote: true, Model: "gpt-image-2"},
		RemoteImageAttempted:    true,
		FallbackUsed:            true,
		PromptTruncated:         true,
		PromptTruncationMessage: "prompt truncated to fit gpt-image-2 prompt limit",
	}
	publicEvidence := model.EvidenceBundle{
		Sources:    []model.SourceSpec{{ID: "src-test", Kind: model.SourceMarkdown, Input: "notes.md"}},
		Items:      []model.EvidenceItem{{ID: "item-test", SourceID: "src-test", Kind: "markdown", Title: "Notes"}},
		Redactions: []model.Redaction{{SourceID: "src-test", Reason: "secret"}},
	}
	warnings := []string{"openai unavailable", "used fallback"}

	audit := baselineAudit(Options{Backend: "hybrid"}, imageResult, publicEvidence, warnings)

	if audit.SelectedBackend != "openai" || audit.RequestedBackend != "hybrid" {
		t.Fatalf("audit backend = %#v, want requested hybrid and selected openai", audit)
	}
	if audit.SourceCount != 1 || audit.EvidenceItemCount != 1 || audit.WarningCount != 2 || audit.RedactionCount != 1 {
		t.Fatalf("audit counts = %#v, want source=1 evidence=1 warning=2 redaction=1", audit)
	}
	if !audit.RemoteImageAttempted || !audit.FallbackUsed || !audit.PromptTruncated {
		t.Fatalf("audit flags = %#v, want remote, fallback, and truncation true", audit)
	}
	if audit.PromptTruncationMessage != imageResult.PromptTruncationMessage {
		t.Fatalf("audit truncation message = %q, want %q", audit.PromptTruncationMessage, imageResult.PromptTruncationMessage)
	}
}

func TestRunRejectsExplicitRemoteBackendWhenOfflineBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "openai",
		Offline:   true,
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want offline/openai rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(statErr) {
		t.Fatalf("scaffold.html exists after rejected run, stat error = %v", statErr)
	}
}

func TestRunRejectsCodexBackendBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	_, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "codex",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want codex rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(statErr) {
		t.Fatalf("scaffold.html exists after rejected run, stat error = %v", statErr)
	}
}

func TestRunSanitizesAbsoluteLocalPathsInOutputs(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "private-repo")
	if err := os.MkdirAll(filepath.Join(repo, "cmd", "server"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Private Repo\n\nSource-backed docs."), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "cmd", "server", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) error = %v", err)
	}
	outputDir := filepath.Join(root, "out")

	_, err := Run(context.Background(), Options{
		Sources:   []string{repo},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, name := range []string{"visual-packet.json", "manifest.json", "scaffold.html"} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		if strings.Contains(string(data), root) {
			t.Fatalf("%s leaked absolute root %q:\n%s", name, root, data)
		}
	}
}

func TestRunLocalHTMLWritesBundleAndManifest(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThe renderer creates a local fallback artifact."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()

	manifest, err := Run(context.Background(), Options{
		Sources:   []string{sourcePath},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Style:     "analytic",
		Goal:      "architecture-map",
		Offline:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, name := range []string{"scaffold.html", "visual-packet.json", "manifest.json", "final.png"} {
		info, err := os.Stat(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}

	if len(manifest.Sources) != 1 {
		t.Fatalf("manifest sources = %#v, want one source", manifest.Sources)
	}
	if manifest.Renderer != "html" {
		t.Fatalf("manifest renderer = %q, want html", manifest.Renderer)
	}
	if manifest.Backend.Name != "local" || manifest.Backend.Remote {
		t.Fatalf("manifest backend = %#v, want local non-remote", manifest.Backend)
	}
	if manifest.Style != "analytic" {
		t.Fatalf("manifest style = %q, want analytic", manifest.Style)
	}
	if len(manifest.Warnings) != 0 {
		t.Fatalf("manifest warnings = %#v, want empty warnings", manifest.Warnings)
	}
	if manifest.Audit.ToolVersion != "0.1.0" {
		t.Fatalf("manifest audit tool version = %q, want 0.1.0", manifest.Audit.ToolVersion)
	}
	if manifest.Audit.RequestedBackend != "local" || manifest.Audit.SelectedBackend != manifest.Backend.Name {
		t.Fatalf("manifest audit backend = %#v, want requested local and selected %q", manifest.Audit, manifest.Backend.Name)
	}
	if manifest.Audit.SourceCount != len(manifest.Sources) || manifest.Audit.EvidenceItemCount == 0 || manifest.Audit.WarningCount != len(manifest.Warnings) {
		t.Fatalf("manifest audit counts = %#v, sources=%d warnings=%d", manifest.Audit, len(manifest.Sources), len(manifest.Warnings))
	}
	if manifest.Audit.RemoteImageAttempted || manifest.Audit.FallbackUsed || manifest.Audit.PromptTruncated {
		t.Fatalf("manifest audit flags = %#v, want false remote/fallback/truncation", manifest.Audit)
	}
	for _, kind := range []string{"scaffold", "visual_packet", "image", "manifest"} {
		if !hasOutputKind(manifest.OutputFiles, kind) {
			t.Fatalf("manifest output files missing kind %q: %#v", kind, manifest.OutputFiles)
		}
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}
	var diskManifest struct {
		Sources     []any                 `json:"sources"`
		Renderer    string                `json:"renderer"`
		Backend     struct{ Name string } `json:"backend"`
		Style       string                `json:"style"`
		Warnings    json.RawMessage       `json:"warnings"`
		Audit       model.ManifestAudit   `json:"audit"`
		OutputFiles []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"output_files"`
	}
	if err := json.Unmarshal(data, &diskManifest); err != nil {
		t.Fatalf("Unmarshal(manifest.json) error = %v", err)
	}
	if len(diskManifest.Sources) != 1 || diskManifest.Renderer != "html" || diskManifest.Backend.Name != "local" || diskManifest.Style != "analytic" {
		t.Fatalf("disk manifest missing required fields: %s", data)
	}
	if len(diskManifest.Warnings) == 0 || string(diskManifest.Warnings) != "[]" {
		t.Fatalf("disk manifest warnings = %s, want [] in %s", diskManifest.Warnings, data)
	}
	if diskManifest.Audit.ToolVersion != "0.1.0" || diskManifest.Audit.RequestedBackend != "local" || diskManifest.Audit.SelectedBackend != "local" {
		t.Fatalf("disk manifest audit missing baseline fields: %s", data)
	}
	if len(diskManifest.OutputFiles) < 4 {
		t.Fatalf("disk manifest output_files = %#v, want at least four files", diskManifest.OutputFiles)
	}
}

func TestRunAddsLocalNextSteps(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "notes.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertNextStepContains(t, manifest.NextSteps, "scaffold.html")
	assertNextStepContains(t, manifest.NextSteps, "visual-packet.json")
	assertNextStepContains(t, manifest.NextSteps, "--handoff codex")
	assertNextStepContains(t, manifest.NextSteps, "--backend openai --renderer image")
}

func TestRunWritesCodexHandoffPackage(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "research-knowledge-base.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Handoff:   "codex",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, rel := range []string{"handoff/codex-prompt.md", "handoff/image-brief.md", "handoff/qa-checklist.md", "handoff/style.md"} {
		info, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %o, want 0600", rel, info.Mode().Perm())
		}
	}
	for _, kind := range []string{"handoff_prompt", "handoff_brief", "handoff_checklist", "handoff_style"} {
		if !hasOutputKind(manifest.OutputFiles, kind) {
			t.Fatalf("manifest output files missing kind %q: %#v", kind, manifest.OutputFiles)
		}
	}
	assertNextStepContains(t, manifest.NextSteps, "handoff/codex-prompt.md")
	assertNoNextStepContains(t, manifest.NextSteps, "--backend codex")
}

func TestRunRejectsQuickWithoutCodexHandoff(t *testing.T) {
	outputDir := t.TempDir()
	_, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "notes.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Quick:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "--quick requires --handoff codex") {
		t.Fatalf("Run() error = %v, want quick handoff error", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(statErr) {
		t.Fatalf("scaffold.html exists after rejected quick run, stat error = %v", statErr)
	}
}

func TestRunAddsOfflineNextStep(t *testing.T) {
	outputDir := t.TempDir()
	manifest, err := Run(context.Background(), Options{
		Sources:   []string{filepath.Join("..", "..", "testdata", "notes.md")},
		OutputDir: outputDir,
		Backend:   "local",
		Renderer:  "html",
		Offline:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertNextStepContains(t, manifest.NextSteps, "offline")
}

func TestNextStepsDescribeOpenAIAndFallbackRuns(t *testing.T) {
	openAISteps := nextSteps(Options{}, imageGenerationResult{
		BackendInfo: model.BackendInfo{Name: "openai", Remote: true, Model: "gpt-image-2"},
	})
	assertNextStepContains(t, openAISteps, "final.png")
	assertNextStepContains(t, openAISteps, "manifest.json")

	fallbackSteps := nextSteps(Options{}, imageGenerationResult{
		BackendInfo:  model.BackendInfo{Name: "local", Remote: false},
		FallbackUsed: true,
	})
	assertNextStepContains(t, fallbackSteps, "fallback")
	assertNextStepContains(t, fallbackSteps, "--backend openai")
	assertNextStepContains(t, fallbackSteps, "--handoff codex")
	assertNoNextStepContains(t, fallbackSteps, "--backend codex")
}

func assertNextStepContains(t *testing.T, steps []string, want string) {
	t.Helper()
	for _, step := range steps {
		if strings.Contains(step, want) {
			return
		}
	}
	t.Fatalf("next_steps missing %q in %#v", want, steps)
}

func assertNoNextStepContains(t *testing.T, steps []string, want string) {
	t.Helper()
	for _, step := range steps {
		if strings.Contains(step, want) {
			t.Fatalf("next_steps unexpectedly contains %q in %#v", want, steps)
		}
	}
}

func hasOutputKind(files []model.OutputFile, kind string) bool {
	return slices.ContainsFunc(files, func(file model.OutputFile) bool {
		return file.Kind == kind
	})
}

func countOutputKind(files []model.OutputFile, kind string) int {
	count := 0
	for _, file := range files {
		if file.Kind == kind {
			count++
		}
	}
	return count
}

func useFakePDFToText(t *testing.T, version string, text string, stderr []byte) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fake pdftotext is Unix-only")
	}
	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, "pdftotext")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"-v\" ]; then\n" +
		"  printf '%s\\n' " + shellLiteral(version) + " >&2\n" +
		"  exit 0\n" +
		"fi\n" +
		"printf '%s\\n' " + shellLiteral(text) + "\n"
	if len(stderr) > 0 {
		script += "printf '%s\\n' " + shellLiteral(string(stderr)) + " >&2\n"
	}
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile(fake pdftotext) error = %v", err)
	}
	t.Setenv("PATH", binDir)
}

func shellLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

type fakeOpenAIBackend struct {
	generateErr error
}

func (fakeOpenAIBackend) Name() string {
	return "openai"
}

func (fakeOpenAIBackend) Available(context.Context) backend.Capability {
	return backend.Capability{
		Available: true,
		Name:      "openai",
		Reason:    "fake OpenAI backend is available",
		Remote:    true,
	}
}

func (f fakeOpenAIBackend) Generate(context.Context, backend.ImageRequest) error {
	return f.generateErr
}

func useOpenAIFake(t *testing.T, fake openAIImageBackend) {
	t.Helper()
	original := newOpenAIBackend
	newOpenAIBackend = func() openAIImageBackend {
		return fake
	}
	t.Cleanup(func() {
		newOpenAIBackend = original
	})
}

func hasWarningContaining(warnings []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), needle) {
			return true
		}
	}
	return false
}
