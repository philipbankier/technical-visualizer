package handoff

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type Options struct {
	OutputImagePath string
}

type Result struct {
	PromptPath    string
	BriefPath     string
	ChecklistPath string
	StylePath     string
}

type handoffFile struct {
	path    string
	content string
}

type privateTarget struct {
	path    string
	content string
}

func WriteCodexPackage(outputDir string, packet model.VisualPacket, opts Options) (Result, error) {
	outputImagePath, err := cleanOutputImagePath(opts.OutputImagePath)
	if err != nil {
		return Result{}, err
	}
	opts.OutputImagePath = outputImagePath

	result := Result{
		PromptPath:    filepath.ToSlash(filepath.Join("handoff", "codex-prompt.md")),
		BriefPath:     filepath.ToSlash(filepath.Join("handoff", "image-brief.md")),
		ChecklistPath: filepath.ToSlash(filepath.Join("handoff", "qa-checklist.md")),
		StylePath:     filepath.ToSlash(filepath.Join("handoff", "style.md")),
	}
	files := []handoffFile{
		{path: result.PromptPath, content: promptMarkdown(packet, opts)},
		{path: result.BriefPath, content: briefMarkdown(packet)},
		{path: result.ChecklistPath, content: checklistMarkdown(opts)},
		{path: result.StylePath, content: styleMarkdown(packet)},
	}

	targets := make([]privateTarget, 0, len(files))
	for _, file := range files {
		target, err := preparePrivateTarget(outputDir, file)
		if err != nil {
			return Result{}, err
		}
		targets = append(targets, target)
	}

	if err := writePreparedTargets(targets); err != nil {
		return Result{}, err
	}

	return result, nil
}

func preparePrivateTarget(outputDir string, file handoffFile) (privateTarget, error) {
	cleanRelPath, err := cleanPackageRelPath(file.path)
	if err != nil {
		return privateTarget{}, err
	}
	path := filepath.Join(outputDir, filepath.FromSlash(cleanRelPath))
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return privateTarget{}, err
	}
	if err := ensurePrivateDirectory(dir); err != nil {
		return privateTarget{}, err
	}
	if err := rejectUnsafeExistingTarget(path); err != nil {
		return privateTarget{}, err
	}
	return privateTarget{path: path, content: file.content}, nil
}

func writePreparedTargets(targets []privateTarget) error {
	rollbacks := make([]func(), 0, len(targets))
	for _, target := range targets {
		rollback, err := writePreparedTarget(target)
		if err != nil {
			rollbackWrittenTargets(rollbacks)
			return err
		}
		rollbacks = append(rollbacks, rollback)
	}
	return nil
}

func writePreparedTarget(target privateTarget) (func(), error) {
	backup, err := backupExistingTarget(target.path)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(target.path)
	file, err := os.CreateTemp(dir, ".codex-handoff-*")
	if err != nil {
		return nil, err
	}
	tmpPath := file.Name()
	defer os.Remove(tmpPath)

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	if _, err := file.WriteString(target.content); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, target.path); err != nil {
		return nil, err
	}
	if err := os.Chmod(target.path, 0o600); err != nil {
		backup.rollback()
		return nil, err
	}
	return backup.rollback, nil
}

type targetBackup struct {
	path    string
	exists  bool
	data    []byte
	mode    os.FileMode
	hasMode bool
}

func backupExistingTarget(target string) (targetBackup, error) {
	backup := targetBackup{path: target}
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return backup, nil
	}
	if err != nil {
		return targetBackup{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return targetBackup{}, fmt.Errorf("handoff target is a symlink: %s", target)
	}
	if !info.Mode().IsRegular() {
		return targetBackup{}, fmt.Errorf("handoff target is not a regular file: %s", target)
	}
	// #nosec G304 -- target was preflighted as a generated handoff file in the output bundle.
	data, err := os.ReadFile(target)
	if err != nil {
		return targetBackup{}, err
	}
	backup.exists = true
	backup.data = data
	backup.mode = info.Mode().Perm()
	backup.hasMode = true
	return backup, nil
}

func rollbackWrittenTargets(rollbacks []func()) {
	for i := len(rollbacks) - 1; i >= 0; i-- {
		rollbacks[i]()
	}
}

func (backup targetBackup) rollback() {
	if backup.exists {
		mode := os.FileMode(0o600)
		if backup.hasMode {
			mode = backup.mode
		}
		_ = os.WriteFile(backup.path, backup.data, mode)
		_ = os.Chmod(backup.path, mode)
		return
	}
	_ = os.Remove(backup.path)
}

func cleanOutputImagePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "final.png", nil
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	if !isSafeOutputImagePathText(normalized) {
		return "", fmt.Errorf("output image path contains unsupported characters: %q", value)
	}
	if strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, "//") {
		return "", fmt.Errorf("output image path must be bundle-relative: %q", value)
	}
	if strings.HasPrefix(normalized, "~/") || normalized == "~" {
		return "", fmt.Errorf("output image path must be bundle-relative: %q", value)
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return "", fmt.Errorf("output image path must be bundle-relative: %q", value)
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return "", fmt.Errorf("output image path must not traverse parents: %q", value)
		}
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("output image path must name a file in the bundle: %q", value)
	}
	return cleaned, nil
}

func isSafeOutputImagePathText(value string) bool {
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '/', '.', '-', '_':
			continue
		default:
			return false
		}
	}
	return true
}

func cleanPackageRelPath(value string) (string, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" || path.IsAbs(value) {
		return "", fmt.Errorf("handoff path must be bundle-relative: %q", value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("handoff path must stay in the bundle: %q", value)
	}
	return cleaned, nil
}

func ensurePrivateDirectory(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("handoff directory is a symlink: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("handoff path is not a directory: %s", dir)
	}
	// #nosec G302 -- generated handoff directories need owner-only execute permission.
	return os.Chmod(dir, 0o700)
}

func rejectUnsafeExistingTarget(target string) error {
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("handoff target is a symlink: %s", target)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("handoff target is not a regular file: %s", target)
	}
	return nil
}

func promptMarkdown(packet model.VisualPacket, opts Options) string {
	return strings.Join([]string{
		"# Codex Image Generation Prompt",
		"",
		"This is an agent handoff workflow, not `visualize --backend codex`.",
		"Use `visual-packet.json`, `scaffold.html`, `handoff/image-brief.md`, `handoff/qa-checklist.md`, and `handoff/style.md` as source material.",
		"Generate one polished single-image technical infographic and save it as `" + opts.OutputImagePath + "`.",
		"",
		"Do not send private source-derived content to remote agents or image tools unless acceptable.",
		"Preserve required text exactly. Do not invent facts, APIs, numbers, papers, or dates. Show uncertainty visibly.",
		"",
		"## Source-Backed Packet Content",
		"",
		"Title: " + fallback(packet.Title),
		"Thesis: " + fallback(packet.Thesis),
		"",
		"Required text:",
		bullets(packet.RequiredText),
		"",
		"Claims:",
		bullets(claimTexts(packet.RankedClaims)),
		"",
		"Facts:",
		bullets(factTexts(packet.Facts)),
		"",
		"Metrics:",
		bullets(metricTexts(packet.Metrics)),
		"",
		"Sections:",
		bullets(sectionTexts(packet.Sections)),
		"",
		"Timeline:",
		bullets(timelineTexts(packet.Timeline)),
		"",
		"Entities:",
		bullets(entityTexts(packet.Entities)),
		"",
		"Open questions:",
		bullets(questionTexts(packet.OpenQuestions)),
		"",
		"Constraints:",
		bullets(packet.Constraints),
		"",
		"Source references:",
		bullets(sourceRefTexts(packet.SourceRefs)),
	}, "\n") + "\n"
}

func briefMarkdown(packet model.VisualPacket) string {
	return strings.Join([]string{
		"# Image Brief",
		"",
		"## Top Source-Backed Story",
		"",
		"Title: " + fallback(packet.Title),
		"Thesis: " + fallback(packet.Thesis),
		"",
		"## Suggested Visual Hierarchy",
		"",
		"- Lead with the title and thesis.",
		"- Use ranked claims as primary evidence blocks.",
		"- Use metrics as prominent callouts with readable labels.",
		"- Keep open questions visible as uncertainty, not conclusions.",
		"",
		"## Important Content",
		"",
		"Required text:",
		bullets(packet.RequiredText),
		"",
		"Claims:",
		bullets(claimTexts(packet.RankedClaims)),
		"",
		"Facts:",
		bullets(factTexts(packet.Facts)),
		"",
		"Metrics:",
		bullets(metricTexts(packet.Metrics)),
		"",
		"Sections:",
		bullets(sectionTexts(packet.Sections)),
		"",
		"Content blocks:",
		bullets(contentBlockTexts(packet.ContentBlocks)),
		"",
		"Timeline:",
		bullets(timelineTexts(packet.Timeline)),
		"",
		"Entities:",
		bullets(entityTexts(packet.Entities)),
		"",
		"Tables:",
		tableMarkdown(packet.Tables),
		"",
		"Diagrams:",
		diagramMarkdown(packet.Diagrams),
		"",
		"Risks:",
		bullets(riskTexts(packet.Risks)),
		"",
		"Tradeoffs:",
		bullets(tradeoffTexts(packet.Tradeoffs)),
		"",
		"Unknowns:",
		bullets(unknownTexts(packet.Unknowns)),
		"",
		"Open questions:",
		bullets(questionTexts(packet.OpenQuestions)),
		"",
		"Constraints:",
		bullets(packet.Constraints),
		"",
		"Source references:",
		bullets(sourceRefTexts(packet.SourceRefs)),
		"",
		"## Risks And Style Notes",
		"",
		"- Do not add unsupported claims.",
		"- Make dense evidence readable in one image.",
		"- Match the style profile in `handoff/style.md`.",
	}, "\n") + "\n"
}

func checklistMarkdown(opts Options) string {
	return strings.Join([]string{
		"# QA Checklist",
		"",
		"- Required text is present and readable.",
		"- Factual claims are source-backed by `visual-packet.json`.",
		"- No invented APIs, numbers, papers, dates, or recommendations were added.",
		"- Uncertainty and open questions are visible.",
		"- Text hierarchy works at final image size.",
		"- Final image is saved at `" + opts.OutputImagePath + "`.",
	}, "\n") + "\n"
}

func styleMarkdown(packet model.VisualPacket) string {
	if packet.Style.Name == "" || packet.Style.Name == "executive-dark" {
		return strings.Join([]string{
			"# Style",
			"",
			"Style profile: executive-dark",
			"Renderer: " + fallback(packet.Style.Renderer),
			"",
			"Use a premium modern technical infographic style with a dark base, strong hierarchy, crisp readable text, restrained panels, and vibrant but controlled accents. Keep dense content scannable and avoid decorative clutter.",
		}, "\n") + "\n"
	}
	return strings.Join([]string{
		"# Style",
		"",
		"Style profile: " + inlineText(packet.Style.Name),
		"Renderer: " + fallback(packet.Style.Renderer),
		"",
		"Keep the result source-backed, readable, polished, and dense without burying required text.",
	}, "\n") + "\n"
}

func claimTexts(claims []model.Claim) []string {
	out := make([]string, 0, len(claims))
	for _, claim := range claims {
		out = append(out, appendSources(claim.Text, claim.SourceRefs))
	}
	return out
}

func factTexts(facts []model.Fact) []string {
	out := make([]string, 0, len(facts))
	for _, fact := range facts {
		out = append(out, appendSources(fact.Text, fact.SourceRefs))
	}
	return out
}

func sectionTexts(sections []model.Section) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		text := section.Title
		if section.Summary != "" {
			text += ": " + section.Summary
		}
		if len(section.Items) > 0 {
			text += " Items: " + strings.Join(section.Items, ", ")
		}
		out = append(out, strings.TrimSpace(text))
	}
	return out
}

func contentBlockTexts(blocks []model.ContentBlock) []string {
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		parts := make([]string, 0, 4)
		if block.Title != "" {
			parts = append(parts, block.Title)
		}
		if block.Kind != "" {
			parts = append(parts, "type: "+block.Kind)
		}
		if block.Summary != "" {
			parts = append(parts, block.Summary)
		}
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
		if len(block.Items) > 0 {
			parts = append(parts, "Items: "+strings.Join(block.Items, ", "))
		}
		out = append(out, appendSources(strings.Join(parts, " - "), block.SourceRefs))
	}
	return out
}

func metricTexts(metrics []model.Metric) []string {
	out := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		text := strings.TrimSpace(metric.Label + ": " + strings.TrimSpace(metric.Value+" "+metric.Unit))
		if metric.Context != "" {
			text += " - " + metric.Context
		}
		out = append(out, appendSources(text, metric.SourceRefs))
	}
	return out
}

func timelineTexts(events []model.TimelineEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		text := strings.TrimSpace(event.Date)
		if text != "" {
			text += ": "
		}
		text += event.Label
		if event.Summary != "" {
			text += " - " + event.Summary
		}
		out = append(out, appendSources(text, event.SourceRefs))
	}
	return out
}

func entityTexts(entities []model.Entity) []string {
	out := make([]string, 0, len(entities))
	for _, entity := range entities {
		text := entity.Name
		if entity.Kind != "" {
			text += " (" + entity.Kind + ")"
		}
		if entity.Detail != "" {
			text += ": " + entity.Detail
		}
		out = append(out, appendSources(text, entity.SourceRefs))
	}
	return out
}

func tableMarkdown(tables []model.PacketTable) string {
	if len(tables) == 0 {
		return "- None provided."
	}
	blocks := make([]string, 0, len(tables))
	for _, table := range tables {
		lines := make([]string, 0, 4+len(table.Rows))
		title := table.Title
		if title == "" {
			title = table.ID
		}
		if title != "" {
			lines = append(lines, "- "+appendSources(title, table.SourceRefs))
		}
		if len(table.Headers) > 0 {
			lines = append(lines, markdownTableRow(table.Headers))
			lines = append(lines, markdownSeparatorRow(len(table.Headers)))
		}
		for _, row := range table.Rows {
			lines = append(lines, markdownTableRow(row))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func markdownTableRow(cells []string) string {
	escaped := make([]string, 0, len(cells))
	for _, cell := range cells {
		escaped = append(escaped, strings.ReplaceAll(inlineText(cell), "|", "\\|"))
	}
	return "| " + strings.Join(escaped, " | ") + " |"
}

func markdownSeparatorRow(count int) string {
	if count <= 0 {
		return ""
	}
	cells := make([]string, count)
	for i := range cells {
		cells[i] = "---"
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

func diagramMarkdown(diagrams []model.Diagram) string {
	if len(diagrams) == 0 {
		return "- None provided."
	}
	blocks := make([]string, 0, len(diagrams))
	for _, diagram := range diagrams {
		title := diagram.Title
		if title == "" {
			title = diagram.ID
		}
		if diagram.Kind != "" {
			title += " (" + diagram.Kind + ")"
		}
		blocks = append(blocks, strings.Join([]string{
			"- " + appendSources(title, diagram.SourceRefs),
			fencedTextBlock(diagram.Text),
		}, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func fencedTextBlock(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	fence := strings.Repeat("`", maxBacktickRun(text)+1)
	if len(fence) < 3 {
		fence = "```"
	}
	return fence + "text\n" + text + "\n" + fence
}

func maxBacktickRun(text string) int {
	maxRun := 0
	current := 0
	for _, r := range text {
		if r == '`' {
			current++
			if current > maxRun {
				maxRun = current
			}
			continue
		}
		current = 0
	}
	return maxRun
}

func questionTexts(questions []model.OpenQuestion) []string {
	out := make([]string, 0, len(questions))
	for _, question := range questions {
		text := question.Text
		if question.Context != "" {
			text += " - " + question.Context
		}
		out = append(out, appendSources(text, question.SourceRefs))
	}
	return out
}

func riskTexts(risks []model.Risk) []string {
	out := make([]string, 0, len(risks))
	for _, risk := range risks {
		text := risk.Text
		if risk.Severity != "" {
			text = risk.Severity + ": " + text
		}
		out = append(out, appendSources(text, risk.SourceRefs))
	}
	return out
}

func tradeoffTexts(tradeoffs []model.Tradeoff) []string {
	out := make([]string, 0, len(tradeoffs))
	for _, tradeoff := range tradeoffs {
		text := tradeoff.Choice
		if tradeoff.Reason != "" {
			text += ": " + tradeoff.Reason
		}
		out = append(out, appendSources(text, tradeoff.SourceRefs))
	}
	return out
}

func unknownTexts(unknowns []model.Unknown) []string {
	out := make([]string, 0, len(unknowns))
	for _, unknown := range unknowns {
		out = append(out, appendSources(unknown.Text, unknown.SourceRefs))
	}
	return out
}

func sourceRefTexts(refs []model.SourceRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		text := ref.ID
		if ref.Label != "" {
			text += ": " + ref.Label
		}
		if ref.Locator != "" {
			text += " - " + ref.Locator
		}
		if ref.SourceID != "" && ref.SourceID != ref.ID {
			text += " (source: " + ref.SourceID + ")"
		}
		out = append(out, strings.TrimSpace(text))
	}
	return out
}

func appendSources(text string, sourceRefs []string) string {
	text = inlineText(text)
	if len(sourceRefs) == 0 {
		return text
	}
	sources := make([]string, 0, len(sourceRefs))
	for _, sourceRef := range sourceRefs {
		sourceRef = inlineText(sourceRef)
		if sourceRef != "" {
			sources = append(sources, sourceRef)
		}
	}
	if len(sources) == 0 {
		return text
	}
	return fmt.Sprintf("%s [source: %s]", text, strings.Join(sources, ", "))
}

func bullets(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		item = inlineText(item)
		if item != "" {
			lines = append(lines, "- "+item)
		}
	}
	if len(lines) == 0 {
		return "- None provided."
	}
	return strings.Join(lines, "\n")
}

func fallback(value string) string {
	value = inlineText(value)
	if value == "" {
		return "Not provided."
	}
	return value
}

func inlineText(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}
