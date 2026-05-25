package source

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

var safePathPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func gatherGitHubRepo(ctx context.Context, spec model.SourceSpec, opts GatherOptions) gatherResult {
	if opts.Offline {
		return gatherResult{Warnings: []string{sourceWarning(spec, "offline mode skipped GitHub repo %q", sourceTarget(spec))}}
	}

	cloneURL, safeID, ok := parseGitHubCloneTarget(sourceTarget(spec))
	if !ok {
		return gatherResult{Warnings: []string{sourceWarning(spec, "invalid GitHub repo URL %q", sourceTarget(spec))}}
	}

	cacheDir, err := githubCacheDir(opts)
	if err != nil {
		return gatherResult{Warnings: []string{sourceWarning(spec, "failed to prepare GitHub cache: %v", err)}}
	}
	repoDir := filepath.Join(cacheDir, safeID)

	if info, err := os.Stat(repoDir); err == nil {
		if !info.IsDir() {
			return gatherResult{Warnings: []string{sourceWarning(spec, "GitHub cache path %q exists but is not a directory", repoDir)}}
		}
		if warning := validateGitHubCache(repoDir, cloneURL); warning != "" {
			return gatherResult{Warnings: []string{sourceWarning(spec, "%s", warning)}}
		}
	} else if os.IsNotExist(err) {
		if warning := cloneGitHubRepo(ctx, cloneURL, repoDir, safeID, opts); warning != "" {
			return gatherResult{Warnings: []string{sourceWarning(spec, "%s", warning)}}
		}
	} else {
		return gatherResult{Warnings: []string{sourceWarning(spec, "failed to inspect GitHub cache path %q: %v", repoDir, err)}}
	}

	localSpec := spec
	localSpec.Resolved = repoDir
	return gatherLocalRepo(ctx, localSpec, opts)
}

func cloneGitHubRepo(ctx context.Context, cloneURL string, repoDir string, safeID string, opts GatherOptions) string {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return fmt.Sprintf("git executable is unavailable; skipped clone for %q", cloneURL)
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(repoDir), "."+safeID+"-tmp-")
	if err != nil {
		return fmt.Sprintf("failed to create temporary clone directory for %q: %v", cloneURL, err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()

	opCtx, cancel := operationContext(ctx, opts)
	defer cancel()

	// #nosec G204 -- git path is resolved with LookPath and cloneURL is constrained to github.com owner/repo HTTPS URLs.
	cmd := exec.CommandContext(opCtx, gitPath, "clone", "--depth", "1", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("git clone failed for %q: %v: %s", cloneURL, err, strings.TrimSpace(string(output)))
	}
	if warning := validateGitHubCache(tempDir, cloneURL); warning != "" {
		return warning
	}
	if err := os.Rename(tempDir, repoDir); err != nil {
		return fmt.Sprintf("failed to finalize GitHub cache for %q: %v", cloneURL, err)
	}
	cleanupTemp = false
	return ""
}

func validateGitHubCache(repoDir string, cloneURL string) string {
	gitDir := filepath.Join(repoDir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return fmt.Sprintf("GitHub cache %q is missing .git directory; skipped reuse", repoDir)
	}
	if !info.IsDir() {
		return fmt.Sprintf("GitHub cache %q has non-directory .git path; skipped reuse", repoDir)
	}

	origin, ok, err := readCachedGitOrigin(repoDir)
	if err != nil {
		return fmt.Sprintf("failed to read GitHub cache origin for %q: %v", repoDir, err)
	}
	if !ok {
		return fmt.Sprintf("GitHub cache %q is missing origin URL %q; skipped reuse", repoDir, cloneURL)
	}
	if !sameGitOrigin(origin, cloneURL) {
		return fmt.Sprintf("GitHub cache %q origin %q does not match %q; skipped reuse", repoDir, origin, cloneURL)
	}
	return ""
}

func readCachedGitOrigin(repoDir string) (string, bool, error) {
	// #nosec G304 -- repoDir is the validated tool-owned GitHub cache directory for this clone target.
	data, err := os.ReadFile(filepath.Join(repoDir, ".git", "config"))
	if err != nil {
		return "", false, err
	}

	inOrigin := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case remoteOriginSectionPattern.MatchString(trimmed):
			inOrigin = true
		case strings.HasPrefix(trimmed, "["):
			inOrigin = false
		case inOrigin && strings.HasPrefix(strings.ToLower(trimmed), "url"):
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) != 2 {
				continue
			}
			return strings.TrimSpace(parts[1]), true, nil
		}
	}
	return "", false, nil
}

func parseGitHubCloneTarget(raw string) (string, string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", "", false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	if repo == "" {
		return "", "", false
	}

	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
	safeID := safePathPattern.ReplaceAllString(owner+"-"+repo+"-"+stableID(cloneURL), "-")
	return cloneURL, safeID, true
}

var remoteOriginSectionPattern = regexp.MustCompile(`^\[remote\s+"origin"\]$`)

func sameGitOrigin(a string, b string) bool {
	return canonicalGitOrigin(a) == canonicalGitOrigin(b)
}

func canonicalGitOrigin(raw string) string {
	canonical := strings.TrimSpace(raw)
	canonical = strings.TrimSuffix(canonical, "/")
	canonical = strings.TrimSuffix(canonical, ".git")
	if strings.HasPrefix(canonical, "git@github.com:") {
		canonical = "https://github.com/" + strings.TrimPrefix(canonical, "git@github.com:")
	}
	parsed, err := url.Parse(canonical)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		parsed.RawQuery = ""
		parsed.Fragment = ""
		canonical = parsed.String()
	}
	return canonical
}

func githubCacheDir(opts GatherOptions) (string, error) {
	dir := opts.CacheDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "technical-visualizer-github-cache")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
