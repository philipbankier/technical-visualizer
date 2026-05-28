package packet

import (
	"errors"
	"fmt"
	"net/url"
	slashpath "path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/philipbankier/technical-visualizer/internal/extract"
	"github.com/philipbankier/technical-visualizer/internal/model"
)

type BuildOptions struct {
	Goal     string
	Audience string
	Style    string
	Renderer string
}

func DefaultBuildOptions() BuildOptions {
	return BuildOptions{Goal: "architecture-map", Audience: "technical decision maker", Style: "executive-dark", Renderer: "hybrid"}
}

func Build(bundle model.EvidenceBundle, opts BuildOptions) (model.VisualPacket, error) {
	if len(bundle.Sources) == 0 {
		return model.VisualPacket{}, errors.New("packet build requires at least one source")
	}

	opts = normalizeBuildOptions(opts)
	primary := bundle.Sources[0]
	primaryName := sourceName(primary)
	title := "Technical Map: " + primaryName
	labels := sourceLabels(primaryName, bundle)
	extracted := extract.FromEvidence(bundle)

	packet := model.VisualPacket{
		SchemaVersion: "visual-packet/v1",
		ArtifactGoal:  opts.Goal,
		Audience:      opts.Audience,
		Title:         title,
		Thesis:        buildThesis(primaryName, len(bundle.Items)),
		RequiredText:  requiredText(title, labels),
		RankedClaims:  rankedClaims(primaryName, bundle, extracted),
		Facts:         factsFromItems(bundle),
		Sections:      sectionsFromLabels(primaryName, labels),
		ContentBlocks: extracted.ContentBlocks,
		Metrics:       extracted.Metrics,
		Timeline:      extracted.Timeline,
		Entities:      extracted.Entities,
		Tables:        extracted.Tables,
		Diagrams:      extracted.Diagrams,
		OpenQuestions: extracted.OpenQuestions,
		Risks:         risksFromWarnings(bundle.Warnings),
		Unknowns:      unknownsFromWarnings(bundle.Warnings),
		Layout: model.LayoutSpec{
			Format:      "single-image-infographic",
			Orientation: "landscape",
			Regions:     []string{"title", "architecture-map", "claims", "risks", "sources"},
		},
		Style: model.StyleSpec{
			Name:     opts.Style,
			Renderer: opts.Renderer,
		},
		Constraints: []string{
			"Do not invent APIs, metrics, product names, source names, citations, or claims not present in source evidence.",
			"Every factual claim must use listed source refs or be marked as a risk, unknown, or assumption.",
			"Do not include credentials, secrets, or redacted values.",
			"Preserve required text exactly in the final artifact.",
		},
		SourceRefs: sourceRefs(bundle),
	}

	return packet, nil
}

func normalizeBuildOptions(opts BuildOptions) BuildOptions {
	defaults := DefaultBuildOptions()
	if strings.TrimSpace(opts.Goal) == "" {
		opts.Goal = defaults.Goal
	}
	if strings.TrimSpace(opts.Audience) == "" {
		opts.Audience = defaults.Audience
	}
	if strings.TrimSpace(opts.Style) == "" {
		opts.Style = defaults.Style
	}
	if strings.TrimSpace(opts.Renderer) == "" {
		opts.Renderer = defaults.Renderer
	}
	return opts
}

func sourceName(spec model.SourceSpec) string {
	for _, key := range []string{"name", "title", "repo", "repository", "project"} {
		if value := displayNameLabel(spec.Metadata[key], spec); value != "" && !isGenericLabel(value) {
			return value
		}
	}

	target := firstNonEmpty(spec.Resolved, spec.Input, spec.ID)
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return sourceNameFromURL(parsed)
	}

	if value := displayNameLabel(target, spec); value != "" && !isGenericLabel(value) {
		return value
	}
	return cleanLabel(target)
}

func sourceNameFromURL(parsed *url.URL) string {
	if strings.EqualFold(parsed.Hostname(), "github.com") {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return cleanLabel(parts[0] + "/" + strings.TrimSuffix(parts[1], ".git"))
		}
	}

	if base := cleanLabel(strings.TrimSuffix(slashpath.Base(parsed.Path), ".git")); base != "" && base != "." && base != "/" && !isGenericLabel(base) {
		return base
	}
	return cleanLabel(parsed.Hostname())
}

func buildThesis(primaryName string, evidenceCount int) string {
	itemWord := "items"
	if evidenceCount == 1 {
		itemWord = "item"
	}
	return fmt.Sprintf("%s is mapped from the primary source with %d evidence %s, preserving source-backed claims and visible uncertainty.", primaryName, evidenceCount, itemWord)
}

func sourceLabels(primaryName string, bundle model.EvidenceBundle) []string {
	seen := map[string]bool{}
	sources := sourcesByID(bundle.Sources)
	labels := make([]string, 0, 8)
	add := func(label string) {
		label = cleanLabel(label)
		if label == "" || isGenericLabel(label) {
			return
		}
		key := strings.ToLower(label)
		if seen[key] {
			return
		}
		seen[key] = true
		labels = append(labels, label)
	}

	add(primaryName)
	for _, source := range bundle.Sources {
		add(sourceName(source))
	}
	for _, item := range bundle.Items {
		source := sources[item.SourceID]
		add(displayTitleLabel(item.Title, source))
		add(displayPathLabel(item.Path, source))
		add(urlLabel(item.URL))
		for _, key := range []string{"title", "heading", "h1", "name"} {
			add(displayLabel(item.Metadata[key]))
		}
		for _, heading := range markdownHeadings(item.Text) {
			add(heading)
		}
	}

	if len(labels) > 8 {
		return labels[:8]
	}
	return labels
}

func requiredText(title string, labels []string) []string {
	required := []string{title}
	for _, label := range labels {
		if label == title {
			continue
		}
		required = append(required, label)
	}
	return required
}

func rankedClaims(primaryName string, bundle model.EvidenceBundle, extracted extract.Result) []model.Claim {
	sources := sourcesByID(bundle.Sources)
	const maxRankedClaims = 6
	claims := make([]model.Claim, 0, maxRankedClaims)
	for _, metric := range extracted.Metrics {
		claims = append(claims, model.Claim{
			ID:         "claim-" + strconv.Itoa(len(claims)+1),
			Text:       metricClaimText(metric),
			Kind:       "metric",
			Confidence: "source-backed",
			SourceRefs: metric.SourceRefs,
		})
		if len(claims) == 3 {
			break
		}
	}
	for _, item := range bundle.Items {
		label := itemLabel(item, sources[item.SourceID])
		if label == "" {
			continue
		}
		text := fmt.Sprintf("%s supplies %s evidence for %s.", label, readableKind(item.Kind), primaryName)
		if cue := evidenceCue(item); cue != "" {
			text = ensureSentence(fmt.Sprintf("%s describes %s", label, cue))
		}
		claim := model.Claim{
			ID:         "claim-" + strconv.Itoa(len(claims)+1),
			Text:       text,
			Kind:       "evidence",
			Confidence: "source-backed",
			SourceRefs: sourceRefsForItem(item),
		}
		claims = append(claims, claim)
		if len(claims) == maxRankedClaims {
			break
		}
	}
	return claims
}

func metricClaimText(metric model.Metric) string {
	context := cleanClaimContext(metric.Context)
	if context == "" {
		return ensureSentence(metric.Label + " reports " + metric.Value)
	}
	if strings.Contains(context, metric.Value) {
		return ensureSentence(metric.Label + " highlights " + context)
	}
	return ensureSentence(metric.Label + " reports " + metric.Value + " in " + strings.TrimSuffix(context, "."))
}

func cleanClaimContext(context string) string {
	context = strings.TrimSpace(context)
	for {
		cleaned := strings.TrimSpace(stripListMarker(context))
		if cleaned == context {
			return cleaned
		}
		context = cleaned
	}
}

func stripListMarker(text string) string {
	switch {
	case strings.HasPrefix(text, "- "), strings.HasPrefix(text, "* "):
		return text[2:]
	}
	dot := strings.Index(text, ". ")
	if dot <= 0 {
		return text
	}
	for _, r := range text[:dot] {
		if !unicode.IsDigit(r) {
			return text
		}
	}
	return text[dot+2:]
}

func factsFromItems(bundle model.EvidenceBundle) []model.Fact {
	sources := sourcesByID(bundle.Sources)
	facts := make([]model.Fact, 0, min(len(bundle.Items), 8))
	for _, item := range bundle.Items {
		label := itemLabel(item, sources[item.SourceID])
		if label == "" {
			continue
		}
		text := fmt.Sprintf("%s is included as %s evidence.", label, readableKind(item.Kind))
		if cue := evidenceCue(item); cue != "" {
			text = ensureSentence(fmt.Sprintf("%s evidence states %s", label, cue))
		}
		facts = append(facts, model.Fact{
			ID:         "fact-" + strconv.Itoa(len(facts)+1),
			Text:       text,
			SourceRefs: sourceRefsForItem(item),
		})
		if len(facts) == 8 {
			break
		}
	}
	return facts
}

func sectionsFromLabels(primaryName string, labels []string) []model.Section {
	items := append([]string(nil), labels...)
	if len(items) > 6 {
		items = items[:6]
	}
	return []model.Section{
		{
			ID:      "architecture-map",
			Title:   "Architecture Map",
			Summary: primaryName + " source structure, evidence paths, claims, and uncertainty.",
			Items:   items,
		},
		{
			ID:      "sources",
			Title:   "Sources",
			Summary: "Source-backed labels and locators used for audit.",
			Items:   items,
		},
	}
}

func risksFromWarnings(warnings []string) []model.Risk {
	risks := make([]model.Risk, 0, len(warnings))
	for _, warning := range warnings {
		if !warningIsRisk(warning) {
			continue
		}
		risks = append(risks, model.Risk{
			ID:       "risk-" + strconv.Itoa(len(risks)+1),
			Text:     "Evidence risk: " + strings.TrimSpace(warning),
			Severity: warningSeverity(warning),
		})
	}
	return risks
}

func unknownsFromWarnings(warnings []string) []model.Unknown {
	unknowns := make([]model.Unknown, 0, len(warnings))
	for _, warning := range warnings {
		if warningIsRisk(warning) {
			continue
		}
		unknowns = append(unknowns, model.Unknown{
			ID:   "unknown-" + strconv.Itoa(len(unknowns)+1),
			Text: "Uncertainty: " + strings.TrimSpace(warning),
		})
	}
	return unknowns
}

func warningIsRisk(warning string) bool {
	lower := strings.ToLower(warning)
	return strings.Contains(lower, "secret") ||
		strings.Contains(lower, "redact") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "skipped") ||
		strings.Contains(lower, "offline")
}

func warningSeverity(warning string) string {
	lower := strings.ToLower(warning)
	switch {
	case strings.Contains(lower, "secret") || strings.Contains(lower, "redact"):
		return "high"
	case strings.Contains(lower, "failed") || strings.Contains(lower, "timeout"):
		return "medium"
	default:
		return "low"
	}
}

func sourceRefs(bundle model.EvidenceBundle) []model.SourceRef {
	seen := map[string]bool{}
	sources := sourcesByID(bundle.Sources)
	refs := make([]model.SourceRef, 0, len(bundle.Sources)+len(bundle.Items))
	add := func(ref model.SourceRef) {
		if ref.ID == "" || seen[ref.ID] {
			return
		}
		seen[ref.ID] = true
		refs = append(refs, ref)
	}

	for _, source := range bundle.Sources {
		add(model.SourceRef{
			ID:       source.ID,
			SourceID: source.ID,
			Label:    sourceName(source),
			Locator:  firstNonEmpty(source.Resolved, source.Input),
		})
	}
	for _, item := range bundle.Items {
		ids := sourceRefsForItem(item)
		for _, id := range ids {
			add(model.SourceRef{
				ID:       id,
				SourceID: item.SourceID,
				Label:    itemLabel(item, sources[item.SourceID]),
				Locator:  firstNonEmpty(item.Path, item.URL),
			})
		}
	}
	return refs
}

func sourceRefsForItem(item model.EvidenceItem) []string {
	if len(item.SourceRefs) > 0 {
		return append([]string(nil), item.SourceRefs...)
	}
	if item.ID != "" {
		return []string{item.ID}
	}
	if item.SourceID != "" && firstNonEmpty(item.Path, item.URL) != "" {
		return []string{item.SourceID + ":" + firstNonEmpty(item.Path, item.URL)}
	}
	return nil
}

func itemLabel(item model.EvidenceItem, source model.SourceSpec) string {
	titleLabel := displayTitleLabel(item.Title, source)
	pathLabel := displayPathLabel(item.Path, source)
	candidates := []string{titleLabel, pathLabel, urlLabel(item.URL), item.ID}
	if looksLikePath(item.Title) {
		candidates = []string{pathLabel, titleLabel, urlLabel(item.URL), item.ID}
	}
	for _, value := range candidates {
		if label := cleanLabel(value); label != "" && !isGenericLabel(label) {
			return label
		}
	}
	return ""
}

func displayTitleLabel(title string, source model.SourceSpec) string {
	return displayPathAwareLabel(title, source)
}

func displayNameLabel(value string, source model.SourceSpec) string {
	return displayPathAwareLabel(value, source)
}

func displayPathAwareLabel(value string, source model.SourceSpec) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if looksLikePath(value) {
		return displayPathLabel(value, source)
	}
	return cleanLabel(value)
}

func displayLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if looksLikePath(value) {
		return displayPathValueLabel(value)
	}
	return cleanLabel(value)
}

func looksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if isAbsoluteDisplayPath(value) {
		return true
	}
	return strings.Contains(value, "/") || strings.Contains(value, "\\")
}

func displayPathLabel(rawPath string, source model.SourceSpec) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return ""
	}

	if rel := sourceRelativePath(rawPath, source); rel != "" {
		return displayPathValueLabel(rel)
	}
	return displayPathValueLabel(rawPath)
}

func displayPathValueLabel(rawPath string) string {
	base := cleanLabel(portableBaseName(rawPath))
	if base == "" {
		return ""
	}
	if isGenericLabel(base) {
		return "Source path"
	}
	return base
}

func sourceRelativePath(rawPath string, source model.SourceSpec) string {
	if rel := portableSourceRelativePath(rawPath, source); rel != "" {
		return rel
	}

	path := filepath.Clean(rawPath)
	for _, root := range []string{source.Resolved, source.Input} {
		root = strings.TrimSpace(root)
		if root == "" || isHTTPURL(root) || !filepath.IsAbs(root) {
			continue
		}
		cleanRoot := filepath.Clean(root)
		if path == cleanRoot {
			return filepath.Base(path)
		}

		candidates := []string{cleanRoot}
		if filepath.Ext(cleanRoot) != "" {
			candidates = append([]string{filepath.Dir(cleanRoot)}, candidates...)
		}
		for _, candidate := range candidates {
			rel, err := filepath.Rel(candidate, path)
			if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
				continue
			}
			return rel
		}
	}
	return ""
}

func portableSourceRelativePath(rawPath string, source model.SourceSpec) string {
	path := portableCleanPath(rawPath)
	if path == "" || path == "." {
		return ""
	}
	for _, root := range []string{source.Resolved, source.Input} {
		root = strings.TrimSpace(root)
		if root == "" || isHTTPURL(root) {
			continue
		}
		cleanRoot := portableCleanPath(root)
		if cleanRoot == "" || cleanRoot == "." {
			continue
		}

		candidates := []string{cleanRoot}
		if slashpath.Ext(cleanRoot) != "" {
			candidates = append([]string{slashpath.Dir(cleanRoot)}, candidates...)
		}
		for _, candidate := range candidates {
			if equalPortablePath(path, candidate) {
				return portableBaseName(path)
			}
			prefix := strings.TrimRight(candidate, "/") + "/"
			if hasPortablePrefix(path, prefix) {
				return path[len(prefix):]
			}
		}
	}
	return ""
}

func isAbsoluteDisplayPath(rawPath string) bool {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return false
	}
	if filepath.IsAbs(filepath.Clean(trimmed)) {
		return true
	}
	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	return isWindowsDriveAbsolute(normalized) || strings.HasPrefix(normalized, "//")
}

func isWindowsDriveAbsolute(path string) bool {
	return len(path) >= 3 && unicode.IsLetter(rune(path[0])) && path[1] == ':' && path[2] == '/'
}

func portableCleanPath(rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	return slashpath.Clean(path)
}

func portableBaseName(rawPath string) string {
	cleaned := portableCleanPath(rawPath)
	if cleaned == "" || cleaned == "." {
		return ""
	}
	return slashpath.Base(cleaned)
}

func equalPortablePath(a string, b string) bool {
	if hasWindowsDrivePrefix(a) || hasWindowsDrivePrefix(b) {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func hasPortablePrefix(path string, prefix string) bool {
	if hasWindowsDrivePrefix(path) || hasWindowsDrivePrefix(prefix) {
		return strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix))
	}
	return strings.HasPrefix(path, prefix)
}

func hasWindowsDrivePrefix(path string) bool {
	return len(path) >= 2 && unicode.IsLetter(rune(path[0])) && path[1] == ':'
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func urlLabel(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Path == "" {
		return displayLabel(raw)
	}
	if base := displayLabel(parsed.Path); base != "" && base != "." && base != "/" {
		return base
	}
	return parsed.Hostname()
}

func markdownHeadings(text string) []string {
	var headings []string
	for _, line := range strings.Split(text, "\n") {
		heading, ok := markdownHeading(line)
		if !ok {
			continue
		}
		if heading == "" {
			continue
		}
		headings = append(headings, heading)
		if len(headings) == 4 {
			break
		}
	}
	return headings
}

type headingSection struct {
	Heading string
	Body    []string
}

func evidenceCue(item model.EvidenceItem) string {
	sections := markdownSections(item.Text)
	if len(sections) > 0 {
		start := 0
		if len(sections) > 1 {
			start = 1
		}
		for _, section := range sections[start:] {
			heading := cleanLabel(section.Heading)
			snippet := firstMeaningfulSnippet(strings.Join(section.Body, "\n"))
			switch {
			case heading != "" && snippet != "":
				return heading + ": " + snippet
			case heading != "":
				return heading
			case snippet != "":
				return snippet
			}
		}
	}
	return firstMeaningfulSnippet(item.Text)
}

func markdownSections(text string) []headingSection {
	var sections []headingSection
	current := -1
	for _, line := range strings.Split(text, "\n") {
		if heading, ok := markdownHeading(line); ok {
			sections = append(sections, headingSection{Heading: heading})
			current = len(sections) - 1
			continue
		}
		if current >= 0 {
			sections[current].Body = append(sections[current].Body, line)
		}
	}
	return sections
}

func markdownHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	trimmed = strings.TrimLeft(trimmed, "#")
	return strings.TrimSpace(trimmed), true
}

func firstMeaningfulSnippet(text string) string {
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, len(lines))
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts = append(parts, strings.Trim(trimmed, "`*_ "))
	}
	text = strings.Join(parts, " ")
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			return strings.TrimSpace(text[:i+len(string(r))])
		}
	}
	if len(text) <= 140 {
		return text
	}
	cut := strings.LastIndex(text[:140], " ")
	if cut < 48 {
		cut = 140
	}
	return strings.TrimSpace(text[:cut])
}

func ensureSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	switch text[len(text)-1] {
	case '.', '!', '?':
		return text
	default:
		return text + "."
	}
}

func readableKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "source"
	}
	return strings.ReplaceAll(kind, "_", " ")
}

func cleanLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	if unescaped, err := url.QueryUnescape(label); err == nil {
		label = unescaped
	}
	label = strings.Trim(label, "`'\" ")
	label = strings.TrimPrefix(label, "#")
	label = strings.Join(strings.Fields(label), " ")
	if len(label) <= 96 {
		return label
	}
	return strings.TrimSpace(label[:96])
}

func isGenericLabel(label string) bool {
	normalized := strings.ToLower(strings.TrimSpace(label))
	normalized = strings.Trim(normalized, ":/\\.-_ ")
	normalized = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		if unicode.IsSpace(r) || r == '-' || r == '_' {
			return ' '
		}
		return -1
	}, normalized)
	normalized = strings.Join(strings.Fields(normalized), " ")

	switch normalized {
	case "", "source", "sources", "evidence", "map", "diagram", "technical map", "visualization", "artifact", "untitled", "home", "index", "overview", "summary":
		return true
	default:
		return false
	}
}

func sourcesByID(sources []model.SourceSpec) map[string]model.SourceSpec {
	byID := make(map[string]model.SourceSpec, len(sources))
	for _, source := range sources {
		byID[source.ID] = source
	}
	return byID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
