package source

import (
	"context"
	"fmt"
	htmltext "html"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

var (
	anchorPattern = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["']`)
	titlePattern  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	scriptPattern = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagPattern    = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern  = regexp.MustCompile(`\s+`)
)

func gatherRemoteMarkdown(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if opts.Offline {
		return gatherResult{Warnings: []string{sourceWarning(spec, "offline mode skipped remote Markdown %q", sourceTarget(spec))}}
	}

	rawURL := sourceTarget(spec)
	data, truncated, err := httpGetBounded(ctx, rawURL, opts)
	if err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "remote Markdown fetch failed for %q: %v", rawURL, err)}}
	}

	text, redactions := redactLikelySecrets(spec.ID, rawURL, string(data))
	result := gatherResult{
		Items: []model.EvidenceItem{{
			ID:       evidenceID(spec.ID, rawURL),
			SourceID: spec.ID,
			Kind:     "markdown",
			Title:    filepath.Base(rawURL),
			Text:     text,
			URL:      rawURL,
			SHA256:   sha256Hex(data),
		}},
		Redactions: redactions,
	}
	if truncated {
		result.Warnings = append(result.Warnings, sourceWarning(spec, "remote Markdown %q was truncated at MaxBytesPerFile=%d", rawURL, opts.MaxBytesPerFile))
	}
	return result
}

func gatherRemotePDF(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if opts.Offline {
		return gatherResult{Warnings: []string{sourceWarning(spec, "offline mode skipped remote PDF %q", sourceTarget(spec))}}
	}

	rawURL := sourceTarget(spec)
	data, truncated, err := httpGetBounded(ctx, rawURL, opts)
	if err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "remote PDF fetch failed for %q: %v", rawURL, err)}}
	}

	result := gatherResult{Warnings: []string{sourceWarning(spec, "PDF text extraction is not implemented in v0.1 for %q; downloaded %d bounded bytes and emitted no text evidence", rawURL, len(data))}}
	if truncated {
		result.Warnings = append(result.Warnings, sourceWarning(spec, "remote PDF %q was truncated at MaxBytesPerFile=%d", rawURL, opts.MaxBytesPerFile))
	}
	return result
}

func gatherDocsSite(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if opts.Offline {
		return gatherResult{Warnings: []string{sourceWarning(spec, "offline mode skipped docs site %q", sourceTarget(spec))}}
	}

	startURL := sourceTarget(spec)
	start, err := url.Parse(startURL)
	if err != nil || start.Scheme == "" || start.Host == "" {
		return gatherResult{Warnings: []string{sourceWarning(spec, "invalid docs URL %q", startURL)}}
	}

	result := gatherResult{}
	queue := []string{start.String()}
	queued := map[string]bool{start.String(): true}
	seen := map[string]bool{}
	pagesAttempted := 0

	for len(queue) > 0 && pagesAttempted < opts.MaxDocsPages {
		if err := ctx.Err(); err != nil {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "%s", err.Error()))
			return result
		}

		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true
		pagesAttempted++

		data, truncated, err := httpGetBounded(ctx, current, opts)
		if err != nil {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "docs fetch failed for %q: %v", current, err))
			continue
		}

		html := string(data)
		title := extractHTMLTitle(html)
		if title == "" {
			title = current
		}
		text, redactions := redactLikelySecrets(spec.ID, current, stripHTML(html))
		result.Redactions = append(result.Redactions, redactions...)
		result.Items = append(result.Items, model.EvidenceItem{
			ID:       evidenceID(spec.ID, current),
			SourceID: spec.ID,
			Kind:     "docs_page",
			Title:    title,
			Text:     text,
			URL:      current,
			SHA256:   sha256Hex(data),
		})
		if truncated {
			result.Warnings = append(result.Warnings, sourceWarning(spec, "docs page %q was truncated at MaxBytesPerFile=%d", current, opts.MaxBytesPerFile))
		}

		for _, next := range extractSameOriginLinks(current, html, start) {
			if queued[next] || seen[next] {
				continue
			}
			queued[next] = true
			queue = append(queue, next)
		}
	}

	if len(queue) > 0 {
		result.Warnings = append(result.Warnings, sourceWarning(spec, "stopped docs crawl after MaxDocsPages=%d", opts.MaxDocsPages))
	}

	return result
}

func httpGetBounded(ctx context.Context, rawURL string, opts GatherOptions) ([]byte, bool, error) {
	opCtx, cancel := operationContext(ctx, opts)
	defer cancel()

	req, err := http.NewRequestWithContext(opCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", "technical-visualizer/0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, opts.MaxBytesPerFile+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > opts.MaxBytesPerFile {
		return data[:opts.MaxBytesPerFile], true, nil
	}
	return data, false, nil
}

func extractHTMLTitle(html string) string {
	matches := titlePattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return cleanText(matches[1])
}

func stripHTML(html string) string {
	text := scriptPattern.ReplaceAllString(html, " ")
	text = tagPattern.ReplaceAllString(text, " ")
	return cleanText(text)
}

func cleanText(text string) string {
	text = htmltext.UnescapeString(text)
	text = spacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractSameOriginLinks(current string, html string, origin *url.URL) []string {
	currentURL, err := url.Parse(current)
	if err != nil {
		return nil
	}

	var links []string
	for _, match := range anchorPattern.FindAllStringSubmatch(html, -1) {
		if len(match) < 2 {
			continue
		}
		href := strings.TrimSpace(match[1])
		if href == "" || strings.HasPrefix(href, "#") {
			continue
		}
		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}
		resolved := currentURL.ResolveReference(parsed)
		resolved.Fragment = ""
		if sameOrigin(resolved, origin) {
			links = append(links, resolved.String())
		}
	}
	return links
}

func sameOrigin(a *url.URL, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}
