package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	openAIBackendName = "openai"
	openAIImageModel  = "gpt-image-2"
	defaultBaseURL    = "https://api.openai.com"
	defaultSize       = "1536x1024"
	defaultQuality    = "high"
	maxPromptRunes    = 32000
)

type OpenAIConfig struct {
	APIKey           string
	BaseURL          string
	HTTPClient       *http.Client
	OperationTimeout time.Duration
}

type OpenAIBackend struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

func NewOpenAIBackend(config OpenAIConfig) *OpenAIBackend {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return &OpenAIBackend{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: client,
		timeout:    valueOrDefaultDuration(config.OperationTimeout, defaultImageGenerationTimeout),
	}
}

func (b *OpenAIBackend) Name() string {
	return openAIBackendName
}

func (b *OpenAIBackend) Available(context.Context) Capability {
	if b.apiKey == "" {
		return Capability{
			Available: false,
			Name:      b.Name(),
			Reason:    "OPENAI_API_KEY is not set",
			Remote:    true,
		}
	}

	return Capability{
		Available: true,
		Name:      b.Name(),
		Reason:    "OPENAI_API_KEY is configured",
		Remote:    true,
	}
}

func (b *OpenAIBackend) Generate(ctx context.Context, request ImageRequest) error {
	if b.apiKey == "" {
		return errors.New("openai image generation requires OPENAI_API_KEY or configured API key")
	}
	if request.OutputPath == "" {
		return errors.New("openai image generation requires OutputPath")
	}

	prompt, err := buildPrompt(request)
	if err != nil {
		return err
	}

	payload := openAIImageRequest{
		Model:   openAIImageModel,
		Prompt:  prompt,
		Size:    valueOrDefault(request.Size, defaultSize),
		Quality: valueOrDefault(request.Quality, defaultQuality),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	opCtx, cancel := operationContext(ctx, b.timeout)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(opCtx, http.MethodPost, b.baseURL+"/v1/images/generations", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+b.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := b.httpClient.Do(httpRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("openai image generation failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var decoded openAIImageResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return err
	}
	if len(decoded.Data) == 0 || decoded.Data[0].B64JSON == "" {
		return errors.New("openai image generation response did not include data[0].b64_json")
	}

	pngData, err := base64.StdEncoding.DecodeString(decoded.Data[0].B64JSON)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(request.OutputPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(request.OutputPath, pngData, 0o600)
}

type openAIImageRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Size    string `json:"size"`
	Quality string `json:"quality"`
}

type openAIImageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

func buildPrompt(request ImageRequest) (string, error) {
	contentBrief := strings.TrimSpace(request.Prompt)
	scaffoldHTML := strings.TrimSpace(request.ScaffoldHTML)
	if contentBrief == "" && scaffoldHTML == "" {
		return "", errors.New("openai image generation requires a prompt or scaffold HTML")
	}

	var sections []string
	if contentBrief != "" {
		sections = append(sections, "Content brief:\n"+contentBrief)
	}
	if scaffoldHTML != "" {
		sections = append(sections, "Audit scaffold HTML:\n"+scaffoldHTML)
	}

	return capPrompt(strings.Join(sections, "\n\n")), nil
}

func capPrompt(prompt string) string {
	if runeCount(prompt) <= maxPromptRunes {
		return prompt
	}

	marker := "\n\n[truncated to fit gpt-image-2 prompt limit]"
	limit := maxPromptRunes - runeCount(marker)
	if limit <= 0 {
		return takeRunes(prompt, maxPromptRunes)
	}
	return takeRunes(prompt, limit) + marker
}

func runeCount(value string) int {
	return len([]rune(value))
}

func takeRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func valueOrDefaultDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
