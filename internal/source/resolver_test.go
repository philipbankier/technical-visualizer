package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestResolverClassifiesInputs(t *testing.T) {
	tmp := t.TempDir()
	packetPath := filepath.Join(tmp, "packet.json")
	notesPath := filepath.Join(tmp, "notes.md")
	repoPath := filepath.Join(tmp, "repo")

	if err := os.WriteFile(packetPath, []byte(`{"schema_version":"visual-packet/v1"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(packet.json) error = %v", err)
	}
	if err := os.WriteFile(notesPath, []byte("# Notes\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(notes.md) error = %v", err)
	}
	if err := os.Mkdir(repoPath, 0o755); err != nil {
		t.Fatalf("Mkdir(repo) error = %v", err)
	}

	cases := []struct {
		name  string
		input string
		want  model.SourceKind
	}{
		{"github", "https://github.com/acme/app", model.SourceGitHubRepo},
		{"docs", "https://docs.acme.com/guide", model.SourceDocsSite},
		{"markdown-url", "https://example.com/guide.md", model.SourceMarkdown},
		{"pdf-url", "https://example.com/report.pdf", model.SourcePDF},
		{"json", packetPath, model.SourceJSONPacket},
		{"markdown-file", notesPath, model.SourceMarkdown},
		{"local-repo", repoPath, model.SourceLocalRepo},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveOne(tc.input)
			if err != nil {
				t.Fatalf("ResolveOne() error = %v", err)
			}
			if got.Kind != tc.want {
				t.Fatalf("Kind = %q, want %q", got.Kind, tc.want)
			}
			if got.ID == "" {
				t.Fatalf("ID is empty")
			}
		})
	}
}

func TestResolverRejectsUnsupportedScheme(t *testing.T) {
	_, err := ResolveOne("ftp://example.com/file")
	if err == nil {
		t.Fatalf("expected unsupported scheme error")
	}
	if !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Fatalf("error = %q, want unsupported scheme", err.Error())
	}
}

func TestResolverRejectsSecretLikeDirectPaths(t *testing.T) {
	tmp := t.TempDir()
	cases := []string{
		".env",
		".npmrc",
		"id_rsa",
		"credentials.json",
		filepath.Join(".aws", "credentials"),
		filepath.Join("config", "secrets.yaml"),
		"private.pem",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(tmp, name)
			_, err := ResolveOne(input)
			if err == nil {
				t.Fatalf("ResolveOne(%q) error = nil, want secret-like rejection", input)
			}
			if !strings.Contains(err.Error(), "secret-like") {
				t.Fatalf("error = %q, want secret-like explanation", err.Error())
			}
		})
	}
}

func TestResolverClassifiesMissingTypedPaths(t *testing.T) {
	tmp := t.TempDir()
	cases := []struct {
		name string
		file string
		want model.SourceKind
	}{
		{"markdown", "missing.md", model.SourceMarkdown},
		{"json", "packet.json", model.SourceJSONPacket},
		{"pdf", "report.pdf", model.SourcePDF},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := filepath.Join(tmp, tc.file)
			got, err := ResolveOne(input)
			if err != nil {
				t.Fatalf("ResolveOne() error = %v", err)
			}
			if got.Kind != tc.want {
				t.Fatalf("Kind = %q, want %q", got.Kind, tc.want)
			}
			if got.Resolved == "" {
				t.Fatalf("Resolved is empty")
			}
			if got.ID == "" {
				t.Fatalf("ID is empty")
			}
		})
	}
}
