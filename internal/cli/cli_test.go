package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpReturnsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(--help) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: visualize") {
		t.Fatalf("help output missing usage text: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDoctorReportsBackendStatusWithoutRemoteCredentials(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Backend status", "local renderer", "OpenAI Images API", "Codex CLI", "agent workflow", "not a direct image backend"} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q: %q", want, output)
		}
	}
	if !strings.Contains(output, "Poppler pdftotext:") {
		t.Fatalf("doctor output missing Poppler status: %q", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDoctorReportsPopplerUnavailableWithInstallGuidance(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Poppler pdftotext: unavailable", "install Poppler for PDF-only sources"} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q: %q", want, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	got := firstNonEmptyLine("\n  pdftotext version 99.1.0  \nCopyright 2005-2026\n")
	if got != "pdftotext version 99.1.0" {
		t.Fatalf("firstNonEmptyLine() = %q", got)
	}
}

func TestRunVersionReturnsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(--version) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "visualize ") {
		t.Fatalf("version output missing CLI name: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMakeLocalHTMLWritesBundle(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n\nThe CLI writes a visualization bundle."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"make", "--backend", "local", "--renderer", "html", "--style", "analytic", "--offline", "--out", outputDir, sourcePath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(make) code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, name := range []string{"scaffold.html", "visual-packet.json", "manifest.json", "final.png"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Fatalf("Stat(%s) error = %v", name, err)
		}
	}
	if !strings.Contains(stdout.String(), outputDir) {
		t.Fatalf("stdout missing output dir %q: %q", outputDir, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunPackAutoWritesSuccessOutput(t *testing.T) {
	outputDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{
		"make",
		"--backend", "local",
		"--renderer", "html",
		"--pack", "auto",
		"--out", outputDir,
		filepath.Join("..", "..", "testdata", "research-knowledge-base.md"),
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(make --pack auto) code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"content-pack.json", filepath.Join(outputDir, "pack")} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q in: %s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "content-pack.json")); err != nil {
		t.Fatalf("Stat(content-pack.json) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "pack", "linkedin-dense", "brief.md")); err != nil {
		t.Fatalf("Stat(pack brief) error = %v", err)
	}
}

func TestRunUnsupportedPackValueFailsBeforeWrites(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--pack", "carousel", "--out", outputDir, sourcePath}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("Run() code = 0, want unsupported pack failure")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "unsupported pack") {
		t.Fatalf("stderr missing pack explanation: %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outputDir, "content-pack.json")); !os.IsNotExist(err) {
		t.Fatalf("content-pack.json exists after rejected run, stat error = %v", err)
	}
}

func TestRunQuickCodexPrintsManualCommand(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "visual output")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"--backend", "local",
		"--renderer", "html",
		"--handoff", "codex",
		"--quick",
		"--out", outputDir,
		filepath.Join("..", "..", "testdata", "research-knowledge-base.md"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{"Codex handoff:", "POSIX shell:", "codex -C", filepath.Join(outputDir, "handoff", "codex-prompt.md")} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q in: %s", want, got)
		}
	}
	if strings.Contains(got, "$(cat handoff/codex-prompt.md)") {
		t.Fatalf("stdout uses prompt path relative to caller cwd: %s", got)
	}
	if !strings.Contains(got, "$(cat < ") {
		t.Fatalf("stdout should read prompt through POSIX input redirection: %s", got)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "handoff", "codex-prompt.md")); err != nil {
		t.Fatalf("Stat(codex-prompt.md) error = %v", err)
	}
}

func TestRunQuickCodexCommandHandlesDashPrefixedOutputDir(t *testing.T) {
	sourcePath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "research-knowledge-base.md"))
	if err != nil {
		t.Fatalf("Abs(source) error = %v", err)
	}
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", workDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore Chdir(%s) error = %v", oldDir, err)
		}
	})

	outputDir := "-bad"
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"--backend", "local",
		"--renderer", "html",
		"--handoff", "codex",
		"--quick",
		"--out", outputDir,
		sourcePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "$(cat < '-bad") {
		t.Fatalf("stdout should protect dash-prefixed prompt path with input redirection: %s", got)
	}
	if strings.Contains(got, "$(cat '-bad") {
		t.Fatalf("stdout lets cat treat dash-prefixed prompt as an option: %s", got)
	}
}

func TestRunQuickCodexPrintsManualCommandWithUppercaseHandoff(t *testing.T) {
	outputDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"--backend", "local",
		"--renderer", "html",
		"--handoff", "CODEX",
		"--quick",
		"--out", outputDir,
		filepath.Join("..", "..", "testdata", "research-knowledge-base.md"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"Codex handoff:", "POSIX shell:", "codex -C", filepath.Join(outputDir, "handoff", "codex-prompt.md")} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q in: %s", want, got)
		}
	}
}

func TestRunCodexBackendReturnsClearErrorBeforeBundleWrite(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(sourcePath, []byte("# System\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"make", "--backend", "codex", "--out", outputDir, sourcePath}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("Run(make --backend codex) code = 0, want failure")
	}
	stderrOutput := strings.ToLower(stderr.String())
	for _, want := range []string{"codex", "agent workflow", "openai"} {
		if !strings.Contains(stderrOutput, want) {
			t.Fatalf("stderr missing %q: %q", want, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "scaffold.html")); !os.IsNotExist(err) {
		t.Fatalf("scaffold.html exists after rejected codex run, stat error = %v", err)
	}
}

func TestRunUnsupportedSubcommandReturnsClearError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"gather", "--unknown"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("Run(gather --unknown) code = 0, want failure")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "unsupported") {
		t.Fatalf("stderr missing unsupported explanation: %q", stderr.String())
	}
}
