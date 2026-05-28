package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOpenAIBackendGeneratesPNG(t *testing.T) {
	var gotRequest struct {
		Model   string `json:"model"`
		Prompt  string `json:"prompt"`
		Size    string `json:"size"`
		Quality string `json:"quality"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("request path = %q, want /v1/images/generations", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": tinyPNGB64(t)}},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "final.png")
	client := NewOpenAIBackend(OpenAIConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	err := client.Generate(context.Background(), ImageRequest{
		Prompt:       "Map the ingestion and rendering system.",
		ScaffoldHTML: "<html><body>Audit scaffold</body></html>",
		OutputPath:   outputPath,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if gotRequest.Model != "gpt-image-2" {
		t.Fatalf("model = %q, want gpt-image-2", gotRequest.Model)
	}
	if gotRequest.Size != "1536x1024" {
		t.Fatalf("size = %q, want 1536x1024", gotRequest.Size)
	}
	if gotRequest.Quality != "high" {
		t.Fatalf("quality = %q, want high", gotRequest.Quality)
	}
	if !strings.Contains(gotRequest.Prompt, "Map the ingestion and rendering system.") {
		t.Fatalf("prompt missing content brief: %q", gotRequest.Prompt)
	}
	if !strings.Contains(gotRequest.Prompt, "<html><body>Audit scaffold</body></html>") {
		t.Fatalf("prompt missing scaffold html: %q", gotRequest.Prompt)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("generated PNG is empty")
	}
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()
	if _, err := png.Decode(file); err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
}

func TestOpenAIBackendRejectsEmptyPrompt(t *testing.T) {
	client := NewOpenAIBackend(OpenAIConfig{APIKey: "test-key"})

	err := client.Generate(context.Background(), ImageRequest{OutputPath: filepath.Join(t.TempDir(), "final.png")})
	if err == nil {
		t.Fatalf("Generate() error = nil, want empty prompt error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "prompt") {
		t.Fatalf("Generate() error = %q, want prompt explanation", err)
	}
}

func TestOpenAIBackendCapsPromptLength(t *testing.T) {
	var gotRequest struct {
		Prompt string `json:"prompt"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": tinyPNGB64(t)}},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	client := NewOpenAIBackend(OpenAIConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	err := client.Generate(context.Background(), ImageRequest{
		Prompt:       strings.Repeat("a", maxPromptRunes),
		ScaffoldHTML: strings.Repeat("b", 200),
		OutputPath:   filepath.Join(t.TempDir(), "final.png"),
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := len([]rune(gotRequest.Prompt)); got != maxPromptRunes {
		t.Fatalf("prompt length = %d, want %d", got, maxPromptRunes)
	}
	if !strings.Contains(gotRequest.Prompt, "truncated to fit gpt-image-2 prompt limit") {
		t.Fatalf("prompt missing truncation marker")
	}
}

func TestBuildPromptReportsTruncation(t *testing.T) {
	result, err := BuildPrompt(ImageRequest{
		Prompt:       strings.Repeat("a", maxPromptRunes),
		ScaffoldHTML: strings.Repeat("b", 200),
	})
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}
	if !result.Truncated {
		t.Fatalf("BuildPrompt() Truncated = false, want true")
	}
	if result.TruncationMessage == "" {
		t.Fatalf("BuildPrompt() TruncationMessage is empty")
	}
	if got := len([]rune(result.Prompt)); got != maxPromptRunes {
		t.Fatalf("prompt length = %d, want %d", got, maxPromptRunes)
	}
}

func TestBuildPromptAddsPackTargetInstructions(t *testing.T) {
	result, err := BuildPrompt(ImageRequest{
		Prompt:            "Target brief:\n- Required content: system map",
		ScaffoldHTML:      "<html><body>System map</body></html>",
		TargetID:          "social-teaser",
		TargetAspectRatio: "1:1",
	})
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	for _, want := range []string{
		"Create the social-teaser image from this source-backed content pack target.",
		"Preserve required text exactly.",
		"Do not invent facts, APIs, papers, numbers, dates, or recommendations.",
		"Make it visually stunning while respecting the target density, intent, and aspect ratio.",
		"Use target aspect ratio 1:1 as composition guidance only.",
		"Target brief:",
		"<html><body>System map</body></html>",
	} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, result.Prompt)
		}
	}
}

func TestCodexBackendAvailableReportsMissingBinary(t *testing.T) {
	client := NewCodexBackend(CodexConfig{BinaryPath: filepath.Join(t.TempDir(), "missing-codex")})

	capability := client.Available(context.Background())

	if capability.Available {
		t.Fatalf("Available = true, want false")
	}
	if capability.Name != "codex" {
		t.Fatalf("Name = %q, want codex", capability.Name)
	}
	if !strings.Contains(strings.ToLower(capability.Reason), "not found") {
		t.Fatalf("Reason = %q, want not found explanation", capability.Reason)
	}
	if capability.Remote {
		t.Fatalf("Remote = true, want false")
	}
}

func TestCodexBackendAvailableReportsVersion(t *testing.T) {
	binaryPath := writeFakeCodex(t, "codex 0.130.0\n")
	client := NewCodexBackend(CodexConfig{BinaryPath: binaryPath})

	capability := client.Available(context.Background())

	if !capability.Available {
		t.Fatalf("Available = false, want true; reason: %s", capability.Reason)
	}
	if capability.Name != "codex" {
		t.Fatalf("Name = %q, want codex", capability.Name)
	}
	if !strings.Contains(capability.Reason, "codex 0.130.0") {
		t.Fatalf("Reason = %q, want version output", capability.Reason)
	}
	if capability.Remote {
		t.Fatalf("Remote = true, want false")
	}
}

func TestCodexBackendGenerateReportsAgentWorkflowOnly(t *testing.T) {
	client := NewCodexBackend(CodexConfig{})

	err := client.Generate(context.Background(), ImageRequest{OutputPath: filepath.Join(t.TempDir(), "final.png")})
	if err == nil {
		t.Fatalf("Generate() error = nil, want agent workflow error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "agent workflow") {
		t.Fatalf("Generate() error = %q, want agent workflow explanation", err)
	}
}

func TestOperationContextAddsDefaultTimeout(t *testing.T) {
	ctx, cancel := operationContext(context.Background(), defaultOperationTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("operation context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > defaultOperationTimeout {
		t.Fatalf("operation deadline remaining = %s, want within default timeout", remaining)
	}
}

func TestOpenAIBackendUsesLongerGenerationTimeout(t *testing.T) {
	client := NewOpenAIBackend(OpenAIConfig{APIKey: "test-key"})

	if client.timeout != defaultImageGenerationTimeout {
		t.Fatalf("timeout = %s, want %s", client.timeout, defaultImageGenerationTimeout)
	}
}

func TestCodexBackendAvailableUsesDefaultTimeout(t *testing.T) {
	binaryPath := writeSlowCodex(t)
	client := NewCodexBackend(CodexConfig{BinaryPath: binaryPath})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	capability := client.Available(ctx)

	if capability.Available {
		t.Fatalf("Available = true, want false")
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("ctx.Err() = %v, want deadline exceeded", ctx.Err())
	}
	if !strings.Contains(strings.ToLower(capability.Reason), "deadline") && !strings.Contains(strings.ToLower(capability.Reason), "killed") {
		t.Fatalf("Reason = %q, want timeout explanation", capability.Reason)
	}
}

func tinyPNGB64(t *testing.T) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})

	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := png.Encode(encoder, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("base64 Close() error = %v", err)
	}
	return buf.String()
}

func writeFakeCodex(t *testing.T, output string) string {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "codex.bat")
		script := "@echo off\r\n<nul set /p=" + output
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		return path
	}

	path := filepath.Join(dir, "codex")
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(output) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func writeSlowCodex(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "codex.bat")
		script := "@echo off\r\nping -n 6 127.0.0.1 >nul\r\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		return path
	}

	path := filepath.Join(dir, "codex")
	script := "#!/bin/sh\nsleep 5\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
