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
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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
