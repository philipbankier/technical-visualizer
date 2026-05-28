package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

type Result struct {
	ContentBlocks []model.ContentBlock
	Metrics       []model.Metric
	Timeline      []model.TimelineEvent
	Entities      []model.Entity
	Tables        []model.PacketTable
	Diagrams      []model.Diagram
	OpenQuestions []model.OpenQuestion
}

type markdownSection struct {
	Title string
	Body  []string
}

var (
	metricPattern         = regexp.MustCompile(`(?:\+\d+(?:\.\d+)?|\d+/\d+|\d+\s*-\s*\d+K?\s+tokens|\d+\s*-\s*\d+\s+edits|[0-9]+K?\s+tokens)`)
	timelineBulletPattern = regexp.MustCompile(`^[-*]\s+(\d{4}(?:-\d{2})?):\s*(.+)$`)
	numberedTitlePattern  = regexp.MustCompile(`^\d+\.\s+(.+)$`)
)

func FromEvidence(bundle model.EvidenceBundle) Result {
	var result Result
	for i, item := range bundle.Items {
		refs := sourceRefsForItem(item)
		sourceKey := sourceKeyForItem(i, item)
		for _, section := range splitMarkdownSections(item.Text) {
			result.ContentBlocks = append(result.ContentBlocks, contentBlock(section, refs, sourceKey))
			result.Metrics = append(result.Metrics, metricsFromSection(section, refs, sourceKey)...)
			result.Timeline = append(result.Timeline, timelineFromSection(section, refs, sourceKey)...)
			result.Entities = append(result.Entities, entitiesFromSection(section, refs, sourceKey)...)
			result.Tables = append(result.Tables, tablesFromSection(section, refs, sourceKey)...)
			result.Diagrams = append(result.Diagrams, diagramsFromSection(section, refs, sourceKey)...)
			result.OpenQuestions = append(result.OpenQuestions, questionsFromSection(section, refs, sourceKey)...)
		}
	}
	return dedupe(result)
}

func splitMarkdownSections(text string) []markdownSection {
	var sections []markdownSection
	var current markdownSection
	inFence := false

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			current.Body = append(current.Body, line)
			inFence = !inFence
			continue
		}
		if inFence {
			current.Body = append(current.Body, line)
			continue
		}
		title, ok := markdownHeading(line)
		if !ok {
			title, ok = plainQuestionHeading(line)
		}
		if !ok {
			current.Body = append(current.Body, line)
			continue
		}
		if current.Title != "" || hasNonEmptyLine(current.Body) {
			if current.Title == "" {
				current.Title = "Document"
			}
			sections = append(sections, current)
		}
		current = markdownSection{Title: title}
	}
	if current.Title != "" || hasNonEmptyLine(current.Body) {
		if current.Title == "" {
			current.Title = "Document"
		}
		sections = append(sections, current)
	}
	return sections
}

func markdownHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimLeft(trimmed, "#")), true
}

func plainQuestionHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	switch strings.ToLower(trimmed) {
	case "open questions", "questions", "research questions":
		return trimmed, true
	default:
		return "", false
	}
}

func contentBlock(section markdownSection, refs []string, sourceKey string) model.ContentBlock {
	text := strings.Join(section.Body, "\n")
	return model.ContentBlock{
		ID:         stableLocalID("block", sourceKey, section.Title, text),
		Kind:       "section",
		Title:      section.Title,
		Summary:    firstSentence(section.Body),
		Text:       trimText(text, 1200),
		Items:      bullets(section.Body),
		SourceRefs: refs,
	}
}

func metricsFromSection(section markdownSection, refs []string, sourceKey string) []model.Metric {
	text := strings.Join(section.Body, " ")
	matches := metricPattern.FindAllString(text, -1)
	metrics := make([]model.Metric, 0, len(matches))
	for _, match := range matches {
		value := strings.Join(strings.Fields(match), " ")
		context := sentenceContaining(text, value)
		metrics = append(metrics, model.Metric{
			ID:         stableLocalID("metric", sourceKey, section.Title, value, context),
			Label:      section.Title,
			Value:      value,
			Context:    context,
			SourceRefs: refs,
		})
	}
	return metrics
}

func timelineFromSection(section markdownSection, refs []string, sourceKey string) []model.TimelineEvent {
	var events []model.TimelineEvent
	for _, line := range section.Body {
		match := timelineBulletPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 3 {
			continue
		}
		events = append(events, model.TimelineEvent{
			ID:         stableLocalID("time", sourceKey, match[1], match[2]),
			Date:       match[1],
			Label:      firstColonPart(match[2]),
			Summary:    strings.TrimSpace(match[2]),
			SourceRefs: refs,
		})
	}
	return events
}

func entitiesFromSection(section markdownSection, refs []string, sourceKey string) []model.Entity {
	name := numberedTitleName(section.Title)
	if name == "" {
		return nil
	}
	return []model.Entity{{
		ID:         stableLocalID("entity", sourceKey, name),
		Kind:       "paper",
		Name:       name,
		Detail:     trimText(strings.Join(nonEmptyLines(section.Body), "; "), 260),
		SourceRefs: refs,
	}}
}

func tablesFromSection(section markdownSection, refs []string, sourceKey string) []model.PacketTable {
	lines := nonEmptyLines(section.Body)
	var tables []model.PacketTable

	for i := 0; i+1 < len(lines); i++ {
		if !isTableRow(lines[i]) || !isTableSeparator(lines[i+1]) {
			continue
		}
		headers := splitTableRow(lines[i])
		var rows [][]string
		for j := i + 2; j < len(lines) && isTableRow(lines[j]); j++ {
			rows = append(rows, splitTableRow(lines[j]))
			i = j
		}
		tables = append(tables, model.PacketTable{
			ID:         stableLocalID("table", sourceKey, section.Title, strings.Join(headers, "|")),
			Title:      section.Title,
			Headers:    headers,
			Rows:       rows,
			SourceRefs: refs,
		})
	}
	return tables
}

func diagramsFromSection(section markdownSection, refs []string, sourceKey string) []model.Diagram {
	var diagrams []model.Diagram
	var fenced []string
	inFence := false

	for _, line := range section.Body {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				text := strings.Join(fenced, "\n")
				if looksDiagram(text) {
					diagrams = append(diagrams, model.Diagram{
						ID:         stableLocalID("diagram", sourceKey, section.Title, text),
						Title:      section.Title,
						Kind:       "ascii",
						Text:       text,
						SourceRefs: refs,
					})
				}
			}
			inFence = !inFence
			fenced = nil
			continue
		}
		if inFence {
			fenced = append(fenced, line)
		}
	}
	return diagrams
}

func questionsFromSection(section markdownSection, refs []string, sourceKey string) []model.OpenQuestion {
	if !strings.Contains(strings.ToLower(section.Title), "question") {
		return nil
	}
	var questions []model.OpenQuestion
	for _, line := range section.Body {
		text := strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if !strings.Contains(text, "?") {
			continue
		}
		questions = append(questions, model.OpenQuestion{
			ID:         stableLocalID("question", sourceKey, text),
			Text:       text,
			Context:    section.Title,
			SourceRefs: refs,
		})
	}
	return questions
}

func sourceKeyForItem(index int, item model.EvidenceItem) string {
	for _, value := range []string{item.ID, item.SourceID, item.Path, item.URL, item.Title} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "item-" + strconv.Itoa(index)
}

func sourceRefsForItem(item model.EvidenceItem) []string {
	if len(item.SourceRefs) > 0 {
		return append([]string(nil), item.SourceRefs...)
	}
	for _, value := range []string{item.ID, item.SourceID, item.Path, item.URL, item.Title} {
		if strings.TrimSpace(value) != "" {
			return []string{value}
		}
	}
	return nil
}

func bullets(lines []string) []string {
	var items []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			items = append(items, strings.TrimSpace(trimmed[2:]))
		}
		if len(items) == 8 {
			break
		}
	}
	return items
}

func nonEmptyLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func hasNonEmptyLine(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

func firstSentence(lines []string) string {
	text := strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
	if text == "" {
		return ""
	}
	for _, sentence := range splitSentences(text) {
		return trimText(sentence, 180)
	}
	return trimText(text, 180)
}

func sentenceContaining(text string, value string) string {
	for _, sentence := range splitSentences(text) {
		if strings.Contains(sentence, value) {
			return sentence
		}
	}
	return ""
}

func splitSentences(text string) []string {
	var sentences []string
	start := 0
	for i, r := range text {
		if r != '.' && r != '?' && r != '!' {
			continue
		}
		if r == '.' && isDecimalPoint(text, i) {
			continue
		}
		sentence := strings.TrimSpace(text[start : i+len(string(r))])
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		start = i + len(string(r))
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

func isDecimalPoint(text string, index int) bool {
	if index <= 0 || index+1 >= len(text) {
		return false
	}
	return text[index-1] >= '0' && text[index-1] <= '9' && text[index+1] >= '0' && text[index+1] <= '9'
}

func firstColonPart(text string) string {
	parts := strings.SplitN(text, ":", 2)
	return strings.TrimSpace(parts[0])
}

func numberedTitleName(title string) string {
	match := numberedTitlePattern.FindStringSubmatch(strings.TrimSpace(title))
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

func isTableSeparator(line string) bool {
	trimmed := strings.Trim(line, "| ")
	return trimmed != "" && strings.Trim(trimmed, "-: |") == ""
}

func splitTableRow(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func looksDiagram(text string) bool {
	return strings.Contains(text, "->") || strings.Contains(text, "+---") || strings.Contains(text, "[") && strings.Contains(text, "]")
}

func trimText(text string, limit int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit]))
}

func stableLocalID(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "-" + hex.EncodeToString(sum[:])[:12]
}

func dedupe(result Result) Result {
	result.Metrics = dedupeMetrics(result.Metrics)
	result.Entities = dedupeEntities(result.Entities)
	return result
}

func dedupeMetrics(metrics []model.Metric) []model.Metric {
	indexByKey := map[string]int{}
	var out []model.Metric
	for _, metric := range metrics {
		key := metric.Value + "\x00" + metric.Context
		if index, ok := indexByKey[key]; ok {
			out[index].SourceRefs = mergeSourceRefs(out[index].SourceRefs, metric.SourceRefs)
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, metric)
	}
	return out
}

func dedupeEntities(entities []model.Entity) []model.Entity {
	indexByKey := map[string]int{}
	var out []model.Entity
	for _, entity := range entities {
		key := strings.ToLower(entity.Kind + "\x00" + entity.Name)
		if index, ok := indexByKey[key]; ok {
			out[index].SourceRefs = mergeSourceRefs(out[index].SourceRefs, entity.SourceRefs)
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, entity)
	}
	return out
}

func mergeSourceRefs(existing []string, next []string) []string {
	seen := map[string]bool{}
	var merged []string
	for _, refs := range [][]string{existing, next} {
		for _, ref := range refs {
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			merged = append(merged, ref)
		}
	}
	return merged
}
