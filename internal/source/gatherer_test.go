package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestGatherLocalMarkdownSource(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "notes.md")
	if err := os.WriteFile(path, []byte("# Architecture\n\nThe gatherer reads markdown evidence."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	spec := model.SourceSpec{
		ID:       "src-notes",
		Kind:     model.SourceMarkdown,
		Input:    path,
		Resolved: path,
	}

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}

	if bundle.SchemaVersion != "evidence/v1" {
		t.Fatalf("SchemaVersion = %q", bundle.SchemaVersion)
	}
	if len(bundle.Sources) != 1 || bundle.Sources[0].ID != "src-notes" {
		t.Fatalf("Sources = %#v", bundle.Sources)
	}
	item := findItemBySource(bundle, "src-notes")
	if item == nil {
		t.Fatalf("expected evidence item for markdown source")
	}
	if item.Kind != "markdown" {
		t.Fatalf("Kind = %q, want markdown", item.Kind)
	}
	if !strings.Contains(item.Text, "gatherer reads markdown evidence") {
		t.Fatalf("Text = %q", item.Text)
	}
	if item.SourceID != "src-notes" {
		t.Fatalf("SourceID = %q", item.SourceID)
	}
}

func TestGatherLocalRepoScanSafetyDefaults(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, tmp, "README.md", "# Project\n\npublic docs")
	writeTestFile(t, tmp, "package.json", `{"scripts":{"test":"go test ./..."}}`)
	writeTestFile(t, tmp, "api/routes.go", "package api\n\nfunc RegisterRoutes() {}\n")
	writeTestFile(t, tmp, "cmd/server/main.go", "package main\n\nfunc main() {}\n")
	writeTestFile(t, tmp, ".env", "SHOULD_NOT_BE_READ=1")
	writeTestFile(t, tmp, ".git/config", "[remote]\n")
	writeTestFile(t, tmp, "node_modules/lib/index.js", "dependency")
	writeTestFile(t, tmp, "dist/bundle.js", "generated")
	writeTestFile(t, tmp, "build/app.js", "generated")
	writeTestFile(t, tmp, "vendor/lib/lib.go", "package lib")
	writeTestFile(t, tmp, "binary.dat", string([]byte{0, 1, 2, 3, 4}))
	writeTestFile(t, tmp, "large.md", strings.Repeat("x", 80))
	writeTestFile(t, tmp, "docs/config.md", "token = abc12345678901234567890")

	spec := model.SourceSpec{
		ID:       "src-repo",
		Kind:     model.SourceLocalRepo,
		Input:    tmp,
		Resolved: tmp,
	}
	opts := DefaultGatherOptions()
	opts.MaxBytesPerFile = 64

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, opts)
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}

	if !hasPath(bundle, "README.md") {
		t.Fatalf("expected README.md evidence, got %#v", itemPaths(bundle))
	}
	if !hasPath(bundle, "package.json") {
		t.Fatalf("expected package.json evidence, got %#v", itemPaths(bundle))
	}
	if !hasPath(bundle, filepath.Join("api", "routes.go")) {
		t.Fatalf("expected route-like file evidence, got %#v", itemPaths(bundle))
	}
	if !hasPath(bundle, filepath.Join("cmd", "server", "main.go")) {
		t.Fatalf("expected Go command entrypoint evidence, got %#v", itemPaths(bundle))
	}
	for _, path := range []string{".env", filepath.Join(".git", "config"), filepath.Join("node_modules", "lib", "index.js"), filepath.Join("dist", "bundle.js"), filepath.Join("build", "app.js"), filepath.Join("vendor", "lib", "lib.go"), "binary.dat", "large.md"} {
		if hasPath(bundle, path) {
			t.Fatalf("unexpected evidence for skipped path %q", path)
		}
	}
	if !containsWarning(bundle.Warnings, ".env") {
		t.Fatalf("expected .env warning, got %#v", bundle.Warnings)
	}
	if !containsWarning(bundle.Warnings, "large.md") {
		t.Fatalf("expected large file warning, got %#v", bundle.Warnings)
	}
	if len(bundle.Redactions) == 0 {
		t.Fatalf("expected secret redaction record")
	}
	if containsAnyEvidenceText(bundle, "abc12345678901234567890") {
		t.Fatalf("expected likely secret to be redacted from evidence text")
	}
	for _, item := range bundle.Items {
		if item.Path != "" && item.SHA256 == "" {
			t.Fatalf("item %q missing SHA256", item.Path)
		}
	}
}

func TestGatherLocalRepoSkipsSymlinkedFiles(t *testing.T) {
	tmp := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	const outsideSecret = "outside-secret-should-never-be-read"
	if err := os.WriteFile(outside, []byte("# Outside\n\n"+outsideSecret), 0o644); err != nil {
		t.Fatalf("WriteFile(outside) error = %v", err)
	}
	writeTestFile(t, tmp, "README.md", "# Project\n\npublic docs")
	if err := os.Symlink(outside, filepath.Join(tmp, "leak.md")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	spec := model.SourceSpec{
		ID:       "src-repo",
		Kind:     model.SourceLocalRepo,
		Input:    tmp,
		Resolved: tmp,
	}

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	if hasPath(bundle, "leak.md") {
		t.Fatalf("expected symlinked file to be skipped, got %#v", itemPaths(bundle))
	}
	if containsAnyEvidenceText(bundle, outsideSecret) {
		t.Fatalf("symlink target content escaped into evidence")
	}
	if !containsWarning(bundle.Warnings, "symlink") {
		t.Fatalf("expected symlink warning, got %#v", bundle.Warnings)
	}
}

func TestGatherRedactsJSONKeysBearerHeadersAndQuotedSpaces(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, tmp, "config.json", `{
  "api_key": "json-secret-value-1234567890",
  "Authorization": "Bearer bearer-token-value-1234567890",
  "password": "quoted value with spaces 1234567890"
}`)

	spec := model.SourceSpec{
		ID:       "src-repo",
		Kind:     model.SourceLocalRepo,
		Input:    tmp,
		Resolved: tmp,
	}

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	for _, secret := range []string{"json-secret-value-1234567890", "bearer-token-value-1234567890", "quoted value with spaces 1234567890"} {
		if containsAnyEvidenceText(bundle, secret) {
			t.Fatalf("expected %q to be redacted from evidence text %#v", secret, evidenceTexts(bundle))
		}
	}
	if len(bundle.Redactions) == 0 {
		t.Fatalf("expected redaction records")
	}
	if !containsAnyEvidenceText(bundle, "[REDACTED]") {
		t.Fatalf("expected redacted marker in evidence text %#v", evidenceTexts(bundle))
	}
}

func TestGatherDefaultOptionsIncludesOperationTimeout(t *testing.T) {
	opts := DefaultGatherOptions()
	if opts.OperationTimeout != 30*time.Second {
		t.Fatalf("OperationTimeout = %s, want 30s", opts.OperationTimeout)
	}
}

func TestGatherDocsSiteCrawlsSameOriginWithinLimit(t *testing.T) {
	var offOriginRequests int
	offOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offOriginRequests++
		fmt.Fprint(w, "off origin")
	}))
	defer offOrigin.Close()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprintf(w, `<html><head><title>Home</title></head><body><h1>Home</h1><p>Root docs</p><a href="/guide">Guide</a><a href="%s/off">External</a></body></html>`, offOrigin.URL)
		case "/guide":
			fmt.Fprint(w, `<html><head><title>Guide</title></head><body><main>Guide body <a href="/deep">Deep</a></main></body></html>`)
		case "/deep":
			fmt.Fprint(w, `<html><head><title>Deep</title></head><body>Deep body</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	spec := model.SourceSpec{
		ID:       "src-docs",
		Kind:     model.SourceDocsSite,
		Input:    server.URL,
		Resolved: server.URL,
	}
	opts := DefaultGatherOptions()
	opts.MaxDocsPages = 2

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, opts)
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}

	if got := countItemsBySource(bundle, "src-docs"); got != 2 {
		t.Fatalf("docs items = %d, want 2", got)
	}
	if !containsAnyEvidenceText(bundle, "Root docs") || !containsAnyEvidenceText(bundle, "Guide body") {
		t.Fatalf("expected root and guide text, got %#v", evidenceTexts(bundle))
	}
	if containsAnyEvidenceText(bundle, "Deep body") {
		t.Fatalf("crawler ignored MaxDocsPages")
	}
	if offOriginRequests != 0 {
		t.Fatalf("off-origin requests = %d, want 0", offOriginRequests)
	}
}

func TestGatherRemoteMarkdownSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "# Remote\n\nRemote markdown evidence")
	}))
	defer server.Close()

	spec := model.SourceSpec{
		ID:       "src-remote-md",
		Kind:     model.SourceMarkdown,
		Input:    server.URL + "/README.md",
		Resolved: server.URL + "/README.md",
	}

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	item := findItemBySource(bundle, "src-remote-md")
	if item == nil {
		t.Fatalf("expected remote markdown item")
	}
	if item.URL == "" {
		t.Fatalf("remote markdown URL is empty")
	}
	if !strings.Contains(item.Text, "Remote markdown evidence") {
		t.Fatalf("Text = %q", item.Text)
	}
}

func TestGatherPDFProducesEvidenceOrBoundedWarning(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	spec := model.SourceSpec{
		ID:       "src-pdf",
		Kind:     model.SourcePDF,
		Input:    path,
		Resolved: path,
	}

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, DefaultGatherOptions())
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}

	item := findItemBySource(bundle, "src-pdf")
	if item == nil && !containsWarning(bundle.Warnings, "PDF") {
		t.Fatalf("expected PDF text evidence or bounded warning, items=%#v warnings=%#v", bundle.Items, bundle.Warnings)
	}
}

func TestGatherOfflineRemoteSourcesWarnsWithoutNetworkCalls(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, "# Should not fetch")
	}))
	defer server.Close()

	specs := []model.SourceSpec{
		{ID: "src-md", Kind: model.SourceMarkdown, Input: server.URL + "/README.md", Resolved: server.URL + "/README.md"},
		{ID: "src-docs", Kind: model.SourceDocsSite, Input: server.URL, Resolved: server.URL},
		{ID: "src-github", Kind: model.SourceGitHubRepo, Input: "https://github.com/acme/app", Resolved: "https://github.com/acme/app"},
	}
	opts := DefaultGatherOptions()
	opts.Offline = true

	bundle, err := GatherAll(context.Background(), specs, opts)
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("offline mode made %d network requests", requests)
	}
	if len(bundle.Items) != 0 {
		t.Fatalf("offline remote gather produced items: %#v", bundle.Items)
	}
	if len(bundle.Warnings) < len(specs) {
		t.Fatalf("expected one warning per remote source, got %#v", bundle.Warnings)
	}
	if !containsWarning(bundle.Warnings, "offline") {
		t.Fatalf("expected offline warning, got %#v", bundle.Warnings)
	}
}

func TestGatherGitHubExistingCacheRequiresGitOriginMatch(t *testing.T) {
	cacheDir := t.TempDir()
	cloneURL, safeID, ok := parseGitHubCloneTarget("https://github.com/acme/app")
	if !ok {
		t.Fatalf("parseGitHubCloneTarget() failed")
	}
	repoDir := filepath.Join(cacheDir, safeID)
	writeTestFile(t, repoDir, "README.md", "# Cached\n\nthis should not be trusted")
	writeTestFile(t, repoDir, ".git/config", "[remote \"origin\"]\n\turl = https://github.com/other/repo.git\n")

	spec := model.SourceSpec{
		ID:       "src-github",
		Kind:     model.SourceGitHubRepo,
		Input:    "https://github.com/acme/app",
		Resolved: "https://github.com/acme/app",
	}
	opts := DefaultGatherOptions()
	opts.CacheDir = cacheDir

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, opts)
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	if len(bundle.Items) != 0 {
		t.Fatalf("expected invalid cache to be skipped, got %#v", bundle.Items)
	}
	if !containsWarning(bundle.Warnings, "origin") || !containsWarning(bundle.Warnings, cloneURL) {
		t.Fatalf("expected origin mismatch warning for %q, got %#v", cloneURL, bundle.Warnings)
	}
}

func TestGatherGitHubExistingCacheRequiresGitDirectory(t *testing.T) {
	cacheDir := t.TempDir()
	_, safeID, ok := parseGitHubCloneTarget("https://github.com/acme/app")
	if !ok {
		t.Fatalf("parseGitHubCloneTarget() failed")
	}
	repoDir := filepath.Join(cacheDir, safeID)
	writeTestFile(t, repoDir, "README.md", "# Cached\n\nthis should not be trusted")

	spec := model.SourceSpec{
		ID:       "src-github",
		Kind:     model.SourceGitHubRepo,
		Input:    "https://github.com/acme/app",
		Resolved: "https://github.com/acme/app",
	}
	opts := DefaultGatherOptions()
	opts.CacheDir = cacheDir

	bundle, err := GatherAll(context.Background(), []model.SourceSpec{spec}, opts)
	if err != nil {
		t.Fatalf("GatherAll() error = %v", err)
	}
	if len(bundle.Items) != 0 {
		t.Fatalf("expected cache without .git to be skipped, got %#v", bundle.Items)
	}
	if !containsWarning(bundle.Warnings, ".git") {
		t.Fatalf("expected missing .git warning, got %#v", bundle.Warnings)
	}
}

func writeTestFile(t *testing.T, root string, rel string, text string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func findItemBySource(bundle model.EvidenceBundle, sourceID string) *model.EvidenceItem {
	for i := range bundle.Items {
		if bundle.Items[i].SourceID == sourceID {
			return &bundle.Items[i]
		}
	}
	return nil
}

func countItemsBySource(bundle model.EvidenceBundle, sourceID string) int {
	var count int
	for _, item := range bundle.Items {
		if item.SourceID == sourceID {
			count++
		}
	}
	return count
}

func hasPath(bundle model.EvidenceBundle, rel string) bool {
	want := filepath.ToSlash(rel)
	for _, item := range bundle.Items {
		if filepath.ToSlash(item.Path) == want {
			return true
		}
	}
	return false
}

func itemPaths(bundle model.EvidenceBundle) []string {
	paths := make([]string, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		if item.Path != "" {
			paths = append(paths, filepath.ToSlash(item.Path))
		}
	}
	return paths
}

func evidenceTexts(bundle model.EvidenceBundle) []string {
	texts := make([]string, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		texts = append(texts, item.Text)
	}
	return texts
}

func containsAnyEvidenceText(bundle model.EvidenceBundle, needle string) bool {
	for _, item := range bundle.Items {
		if strings.Contains(item.Text, needle) {
			return true
		}
	}
	return false
}

func containsWarning(warnings []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), needle) {
			return true
		}
	}
	return false
}
