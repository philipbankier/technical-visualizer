package packet

import (
	"strings"
	"testing"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

func TestBuildCreatesSourceSpecificVisualPacket(t *testing.T) {
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-acme",
			Kind:     model.SourceLocalRepo,
			Input:    "/work/acme-payments",
			Resolved: "/work/acme-payments",
			Metadata: map[string]string{"name": "Acme Payments API"},
		}},
		Items: []model.EvidenceItem{
			{
				ID:         "ev-readme",
				SourceID:   "src-acme",
				Kind:       "markdown",
				Title:      "README.md",
				Text:       "# Acme Payments API\n\n## Ingestion\n\nSource adapters normalize payment webhooks.",
				Path:       "README.md",
				SourceRefs: []string{"src-acme:README.md"},
			},
			{
				ID:         "ev-routes",
				SourceID:   "src-acme",
				Kind:       "code",
				Title:      "internal/api/routes.go",
				Text:       "RegisterRoutes wires webhook handlers to the renderer queue.",
				Path:       "internal/api/routes.go",
				SourceRefs: []string{"src-acme:internal/api/routes.go"},
			},
			{
				ID:         "ev-mod",
				SourceID:   "src-acme",
				Kind:       "dependency_manifest",
				Title:      "go.mod",
				Text:       "module acme-payments",
				Path:       "go.mod",
				SourceRefs: []string{"src-acme:go.mod"},
			},
		},
		Warnings: []string{
			"source src-acme: skipped secret-like file \".env\"",
			"source src-acme: PDF text extraction is not implemented in v0.1 for \"architecture.pdf\"",
		},
	}
	opts := DefaultBuildOptions()
	opts.Goal = "system-map"
	opts.Audience = "platform architect"
	opts.Style = "analytic"
	opts.Renderer = "html"

	packet, err := Build(bundle, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if packet.SchemaVersion != "visual-packet/v1" {
		t.Fatalf("SchemaVersion = %q", packet.SchemaVersion)
	}
	if !strings.Contains(packet.Title, "Acme Payments API") {
		t.Fatalf("Title = %q, want source-specific name", packet.Title)
	}
	if packet.Title == "Technical Map" || packet.Title == "Technical Map: source" {
		t.Fatalf("Title = %q, generic filler is not acceptable", packet.Title)
	}
	if !strings.Contains(packet.Thesis, "Acme Payments API") || !strings.Contains(packet.Thesis, "3") {
		t.Fatalf("Thesis = %q, want primary source name and evidence count", packet.Thesis)
	}
	if !containsString(packet.RequiredText, packet.Title) {
		t.Fatalf("RequiredText missing title %q: %#v", packet.Title, packet.RequiredText)
	}
	sourceLabelCount := countRequiredLabels(packet.RequiredText, []string{
		"Acme Payments API",
		"README.md",
		"internal/api/routes.go",
		"go.mod",
		"Ingestion",
	})
	if sourceLabelCount < 3 {
		t.Fatalf("RequiredText = %#v, want at least three source-specific labels", packet.RequiredText)
	}
	if len(packet.RankedClaims) == 0 || len(packet.RankedClaims) > 5 {
		t.Fatalf("RankedClaims length = %d, want 1..5", len(packet.RankedClaims))
	}
	for _, claim := range packet.RankedClaims {
		if len(claim.SourceRefs) == 0 {
			t.Fatalf("claim %q missing source refs: %#v", claim.ID, claim)
		}
	}
	if !hasClaimOrFactContaining(packet, "Ingestion") && !hasClaimOrFactContaining(packet, "Source adapters normalize payment webhooks") {
		t.Fatalf("expected claim or fact to include source-derived cue, claims=%#v facts=%#v", packet.RankedClaims, packet.Facts)
	}
	if packet.Style.Name != "analytic" || packet.Style.Renderer != "html" {
		t.Fatalf("Style = %#v, want opts carried through", packet.Style)
	}
	if packet.Layout.Format != "single-image-infographic" {
		t.Fatalf("Layout.Format = %q", packet.Layout.Format)
	}
	if packet.Layout.Orientation != "landscape" {
		t.Fatalf("Layout.Orientation = %q", packet.Layout.Orientation)
	}
	for _, region := range []string{"title", "architecture-map", "claims", "risks", "sources"} {
		if !containsString(packet.Layout.Regions, region) {
			t.Fatalf("Layout.Regions = %#v, missing %q", packet.Layout.Regions, region)
		}
	}
	constraints := strings.ToLower(strings.Join(packet.Constraints, " "))
	for _, required := range []string{"do not invent", "apis", "metrics", "names", "citations", "preserve required text"} {
		if !strings.Contains(constraints, required) {
			t.Fatalf("Constraints = %#v, missing %q", packet.Constraints, required)
		}
	}
	if !hasRiskContaining(packet.Risks, ".env") {
		t.Fatalf("Risks = %#v, want secret/skipped warning with severity", packet.Risks)
	}
	if !hasUnknownContaining(packet.Unknowns, "pdf text extraction") {
		t.Fatalf("Unknowns = %#v, want PDF extraction warning with uncertainty text", packet.Unknowns)
	}
}

func TestBuildDoesNotDisplayAbsoluteLocalPaths(t *testing.T) {
	const privatePath = "/Users/example/private/repo/docs/architecture.md"
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    "/Users/example/private/repo",
			Resolved: "/Users/example/private/repo",
			Metadata: map[string]string{"name": "Private Repo"},
		}},
		Items: []model.EvidenceItem{{
			ID:       "ev-architecture",
			SourceID: "src-private",
			Kind:     "markdown",
			Text:     "# Architecture\n\nSource package boundaries.",
			Path:     privatePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	requiredText := strings.Join(packet.RequiredText, "\n")
	if strings.Contains(requiredText, "/Users/example/private") {
		t.Fatalf("RequiredText leaked private path: %#v", packet.RequiredText)
	}
	if !strings.Contains(requiredText, "docs/architecture.md") && !strings.Contains(requiredText, "architecture.md") {
		t.Fatalf("RequiredText = %#v, want source-relative path or basename", packet.RequiredText)
	}
	if !hasSourceRefLocator(packet.SourceRefs, privatePath) {
		t.Fatalf("SourceRefs = %#v, want raw path preserved in locator", packet.SourceRefs)
	}
}

func TestBuildDoesNotDisplayAbsoluteLocalTitles(t *testing.T) {
	const privatePath = "/Users/example/private/repo/docs/architecture.md"
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    "/Users/example/private/repo",
			Resolved: "/Users/example/private/repo",
			Metadata: map[string]string{"name": "Private Repo"},
		}},
		Items: []model.EvidenceItem{{
			ID:       "ev-architecture",
			SourceID: "src-private",
			Kind:     "markdown",
			Title:    privatePath,
			Text:     "# Architecture\n\nSource package boundaries.",
			Path:     privatePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoPrivatePath(t, "RequiredText", strings.Join(packet.RequiredText, "\n"))
	for _, claim := range packet.RankedClaims {
		assertNoPrivatePath(t, "RankedClaims", claim.Text)
	}
	for _, fact := range packet.Facts {
		assertNoPrivatePath(t, "Facts", fact.Text)
	}
	for _, ref := range packet.SourceRefs {
		assertNoPrivatePath(t, "SourceRefs.Label", ref.Label)
	}
	if !hasSourceRefLocator(packet.SourceRefs, privatePath) {
		t.Fatalf("SourceRefs = %#v, want raw path preserved in locator", packet.SourceRefs)
	}
}

func TestBuildDoesNotDisplayWindowsDriveLetterPaths(t *testing.T) {
	const privateRoot = `C:\Users\example\private`
	const privatePath = `C:\Users\example\private\repo\docs\architecture.md`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    `C:\Users\example\private\repo`,
			Resolved: `C:\Users\example\private\repo`,
			Metadata: map[string]string{"name": "Private Repo"},
		}},
		Items: []model.EvidenceItem{{
			ID:       "ev-architecture",
			SourceID: "src-private",
			Kind:     "markdown",
			Title:    privatePath,
			Text:     "# Architecture\n\nSource package boundaries.",
			Path:     privatePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoDisplayText(t, packet, privateRoot)
	if !hasSourceRefLocator(packet.SourceRefs, privatePath) {
		t.Fatalf("SourceRefs = %#v, want raw Windows path preserved in locator", packet.SourceRefs)
	}
}

func TestBuildDoesNotDisplayUNCPaths(t *testing.T) {
	const privateRoot = `server\share\private`
	const privatePath = `\\server\share\private\repo\docs\architecture.md`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    `\\server\share\private\repo`,
			Resolved: `\\server\share\private\repo`,
			Metadata: map[string]string{"name": "Private Repo"},
		}},
		Items: []model.EvidenceItem{{
			ID:       "ev-architecture",
			SourceID: "src-private",
			Kind:     "markdown",
			Title:    privatePath,
			Text:     "# Architecture\n\nSource package boundaries.",
			Path:     privatePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoDisplayText(t, packet, privateRoot)
	if !hasSourceRefLocator(packet.SourceRefs, privatePath) {
		t.Fatalf("SourceRefs = %#v, want raw UNC path preserved in locator", packet.SourceRefs)
	}
}

func TestBuildDoesNotDisplayWindowsSourceNameFallback(t *testing.T) {
	const privateRoot = `C:\Users\example\private`
	const sourcePath = `C:\Users\example\private\repo`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    sourcePath,
			Resolved: sourcePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoSourceDisplayText(t, packet, privateRoot)
	if !strings.Contains(packet.Title, "repo") {
		t.Fatalf("Title = %q, want safe basename", packet.Title)
	}
}

func TestBuildDoesNotDisplayUNCSourceNameFallback(t *testing.T) {
	const privateRoot = `server\share\private`
	const sourcePath = `\\server\share\private\repo`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    sourcePath,
			Resolved: sourcePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoSourceDisplayText(t, packet, privateRoot)
	if !strings.Contains(packet.Title, "repo") {
		t.Fatalf("Title = %q, want safe basename", packet.Title)
	}
}

func TestBuildSanitizesPathLikeSourceMetadata(t *testing.T) {
	const privateRoot = `C:\Users\example\private`
	const sourcePath = `C:\Users\example\private\repo`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    sourcePath,
			Resolved: sourcePath,
			Metadata: map[string]string{"name": sourcePath},
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoSourceDisplayText(t, packet, privateRoot)
	if !strings.Contains(packet.Title, "repo") {
		t.Fatalf("Title = %q, want safe metadata basename", packet.Title)
	}
}

func TestBuildDoesNotDisplayGenericWindowsSourceNameFallback(t *testing.T) {
	const privateRoot = `C:\Users\example\private`
	const sourcePath = `C:\Users\example\private\overview`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    sourcePath,
			Resolved: sourcePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoSourceDisplayText(t, packet, privateRoot)
	if strings.TrimSpace(packet.Title) == "" || packet.Title == "Technical Map: " {
		t.Fatalf("Title = %q, want safe non-empty title", packet.Title)
	}
}

func TestBuildDoesNotDisplayGenericUNCSourceNameFallback(t *testing.T) {
	const privateRoot = `server\share\private`
	const sourcePath = `\\server\share\private\summary`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    sourcePath,
			Resolved: sourcePath,
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertNoSourceDisplayText(t, packet, privateRoot)
	if strings.TrimSpace(packet.Title) == "" || packet.Title == "Technical Map: " {
		t.Fatalf("Title = %q, want safe non-empty title", packet.Title)
	}
}

func TestBuildSanitizesPathLikeItemMetadataLabels(t *testing.T) {
	const privateRoot = `C:\Users\example\private`
	const privatePath = `C:\Users\example\private\repo\docs\architecture.md`
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-private",
			Kind:     model.SourceLocalRepo,
			Input:    `C:\Users\example\private\repo`,
			Resolved: `C:\Users\example\private\repo`,
			Metadata: map[string]string{"name": "Private Repo"},
		}},
		Items: []model.EvidenceItem{{
			ID:       "ev-architecture",
			SourceID: "src-private",
			Kind:     "markdown",
			Text:     "# Architecture\n\nSource package boundaries.",
			Path:     "docs/architecture.md",
			Metadata: map[string]string{
				"title":   privatePath,
				"heading": privatePath,
			},
		}},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	privateText := strings.ToLower(privateRoot)
	assertNoDisplayTextValue(t, "RequiredText", strings.Join(packet.RequiredText, "\n"), privateText)
	for _, section := range packet.Sections {
		assertNoDisplayTextValue(t, "Sections.Items", strings.Join(section.Items, "\n"), privateText)
	}
}

func TestBuildLimitsRankedClaimsToFive(t *testing.T) {
	bundle := model.EvidenceBundle{
		SchemaVersion: "evidence/v1",
		Sources: []model.SourceSpec{{
			ID:       "src-control-plane",
			Kind:     model.SourceLocalRepo,
			Input:    "/work/control-plane",
			Resolved: "/work/control-plane",
			Metadata: map[string]string{"name": "Control Plane"},
		}},
		Items: []model.EvidenceItem{
			evidenceItem("ev-1", "README.md"),
			evidenceItem("ev-2", "cmd/server/main.go"),
			evidenceItem("ev-3", "internal/api/routes.go"),
			evidenceItem("ev-4", "internal/worker/queue.go"),
			evidenceItem("ev-5", "internal/render/scaffold.go"),
			evidenceItem("ev-6", "internal/quality/quality.go"),
			evidenceItem("ev-7", "go.mod"),
		},
	}

	packet, err := Build(bundle, DefaultBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(packet.RankedClaims) != 5 {
		t.Fatalf("RankedClaims length = %d, want exactly 5", len(packet.RankedClaims))
	}
}

func TestBuildRejectsBundleWithoutSources(t *testing.T) {
	_, err := Build(model.EvidenceBundle{SchemaVersion: "evidence/v1"}, DefaultBuildOptions())
	if err == nil {
		t.Fatalf("Build() error = nil, want error for bundle without sources")
	}
}

func TestDefaultBuildOptions(t *testing.T) {
	opts := DefaultBuildOptions()
	if opts.Goal != "architecture-map" {
		t.Fatalf("Goal = %q", opts.Goal)
	}
	if opts.Audience != "technical decision maker" {
		t.Fatalf("Audience = %q", opts.Audience)
	}
	if opts.Style != "executive-dark" {
		t.Fatalf("Style = %q", opts.Style)
	}
	if opts.Renderer != "hybrid" {
		t.Fatalf("Renderer = %q", opts.Renderer)
	}
}

func evidenceItem(id string, path string) model.EvidenceItem {
	return model.EvidenceItem{
		ID:         id,
		SourceID:   "src-control-plane",
		Kind:       "code",
		Title:      path,
		Path:       path,
		SourceRefs: []string{"src-control-plane:" + path},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func countRequiredLabels(required []string, labels []string) int {
	count := 0
	for _, label := range labels {
		for _, value := range required {
			if value == label || strings.Contains(value, label) {
				count++
				break
			}
		}
	}
	return count
}

func hasClaimOrFactContaining(packet model.VisualPacket, text string) bool {
	for _, claim := range packet.RankedClaims {
		if strings.Contains(claim.Text, text) {
			return true
		}
	}
	for _, fact := range packet.Facts {
		if strings.Contains(fact.Text, text) {
			return true
		}
	}
	return false
}

func hasSourceRefLocator(refs []model.SourceRef, locator string) bool {
	for _, ref := range refs {
		if ref.Locator == locator {
			return true
		}
	}
	return false
}

func assertNoPrivatePath(t *testing.T, field string, text string) {
	t.Helper()
	if strings.Contains(text, "/Users/example/private") {
		t.Fatalf("%s leaked private path in %q", field, text)
	}
}

func assertNoDisplayText(t *testing.T, packet model.VisualPacket, privateText string) {
	t.Helper()
	privateText = strings.ToLower(privateText)
	assertNoDisplayTextValue(t, "RequiredText", strings.Join(packet.RequiredText, "\n"), privateText)
	for _, claim := range packet.RankedClaims {
		assertNoDisplayTextValue(t, "RankedClaims", claim.Text, privateText)
	}
	for _, fact := range packet.Facts {
		assertNoDisplayTextValue(t, "Facts", fact.Text, privateText)
	}
	for _, ref := range packet.SourceRefs {
		assertNoDisplayTextValue(t, "SourceRefs.Label", ref.Label, privateText)
	}
}

func assertNoDisplayTextValue(t *testing.T, field string, text string, privateText string) {
	t.Helper()
	if strings.Contains(strings.ToLower(text), privateText) {
		t.Fatalf("%s leaked private path in %q", field, text)
	}
}

func assertNoSourceDisplayText(t *testing.T, packet model.VisualPacket, privateText string) {
	t.Helper()
	privateText = strings.ToLower(privateText)
	assertNoDisplayTextValue(t, "Title", packet.Title, privateText)
	assertNoDisplayTextValue(t, "Thesis", packet.Thesis, privateText)
	assertNoDisplayTextValue(t, "RequiredText", strings.Join(packet.RequiredText, "\n"), privateText)
	for _, section := range packet.Sections {
		assertNoDisplayTextValue(t, "Sections.Summary", section.Summary, privateText)
		assertNoDisplayTextValue(t, "Sections.Items", strings.Join(section.Items, "\n"), privateText)
	}
	for _, ref := range packet.SourceRefs {
		assertNoDisplayTextValue(t, "SourceRefs.Label", ref.Label, privateText)
	}
}

func hasRiskContaining(risks []model.Risk, text string) bool {
	text = strings.ToLower(text)
	for _, risk := range risks {
		if risk.Severity != "" && strings.Contains(strings.ToLower(risk.Text), text) {
			return true
		}
	}
	return false
}

func hasUnknownContaining(unknowns []model.Unknown, text string) bool {
	text = strings.ToLower(text)
	for _, unknown := range unknowns {
		lower := strings.ToLower(unknown.Text)
		if strings.Contains(lower, text) && strings.Contains(lower, "uncertainty") {
			return true
		}
	}
	return false
}
