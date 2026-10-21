// Package translation provides a small OpenAI-compatible translation client.
package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dujiao-next/internal/config"
)

const APIKeyEnv = "SILICONFLOW_API_KEY"

var (
	ErrDisabled        = errors.New("translation is disabled")
	ErrAPIKeyMissing   = errors.New("translation API key is missing")
	ErrInvalidResponse = errors.New("translation response is invalid")
)

// ProductText is the translatable portion of a catalog product.
type ProductText struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// Translator is the application port used by catalog synchronization.
type Translator interface {
	TranslateProduct(context.Context, ProductText) (ProductText, error)
}

type Client struct {
	enabled   bool
	endpoint  string
	model     string
	maxChars  int
	http      *http.Client
	semaphore chan struct{}
}

// NewClient creates a translator. The secret is intentionally not accepted as
// an argument or read from config; TranslateProduct reads APIKeyEnv at runtime.
func NewClient(cfg config.TranslationConfig) *Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxChars := cfg.MaxChars
	if maxChars <= 0 {
		maxChars = 12000
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = config.DefaultTranslationBaseURL
	}
	if !strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL += "/chat/completions"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = config.DefaultTranslationModel
	}
	return &Client{
		enabled:  cfg.Enabled,
		endpoint: baseURL,
		model:    model,
		maxChars: maxChars,
		http:     &http.Client{Timeout: timeout},
		// Keep the initial translation pass gentle on the small VPS and on the
		// provider rate limit. The semaphore is local to this process.
		semaphore: make(chan struct{}, 1),
	}
}

func (c *Client) TranslateProduct(ctx context.Context, source ProductText) (ProductText, error) {
	if c == nil || !c.enabled {
		return source, ErrDisabled
	}
	if strings.TrimSpace(os.Getenv(APIKeyEnv)) == "" {
		return source, ErrAPIKeyMissing
	}
	if strings.TrimSpace(source.Title) == "" && strings.TrimSpace(source.Description) == "" && strings.TrimSpace(source.Content) == "" {
		return source, nil
	}

	input := truncateProductText(source, c.maxChars)
	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "You are a professional Simplified Chinese translator for a digital-goods storefront. Translate the provided English product content into Simplified Chinese. Preserve Markdown, URLs, placeholders, code, product codes, and line breaks. Do not add facts. Return only valid JSON with exactly the keys title, description, and content; empty input fields must remain empty.",
			},
			{
				Role:    "user",
				Content: mustJSON(input),
			},
		},
		Temperature: 0.1,
		Stream:      false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return source, fmt.Errorf("build translation request: %w", err)
	}

	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return source, ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return source, fmt.Errorf("build translation request: %w", err)
	}
	// The key is used only in the in-memory request header and is never part
	// of an error, log field, config value, or persisted connection record.
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv(APIKeyEnv)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return source, fmt.Errorf("translation request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.CopyN(io.Discard, resp.Body, 1<<20)
		return source, fmt.Errorf("translation provider returned status %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return source, fmt.Errorf("decode translation response: %w", err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return source, ErrInvalidResponse
	}
	translated, err := parseProductText(result.Choices[0].Message.Content)
	if err != nil {
		return source, err
	}
	return fillMissing(source, translated), nil
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func mustJSON(value ProductText) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func parseProductText(content string) (ProductText, error) {
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "<think>"); start >= 0 {
		if end := strings.Index(content[start+len("<think>"):], "</think>"); end >= 0 {
			content = content[:start] + content[start+len("<think>")+end+len("</think>"):]
		}
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:]
		}
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	}
	content = strings.TrimSpace(content)
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start < 0 || end < start {
		return ProductText{}, ErrInvalidResponse
	}
	var translated ProductText
	if err := json.Unmarshal([]byte(content[start:end+1]), &translated); err != nil {
		return ProductText{}, fmt.Errorf("parse translation JSON: %w", err)
	}
	return translated, nil
}

func fillMissing(source, translated ProductText) ProductText {
	if strings.TrimSpace(translated.Title) == "" {
		translated.Title = source.Title
	}
	if strings.TrimSpace(translated.Description) == "" {
		translated.Description = source.Description
	}
	if strings.TrimSpace(translated.Content) == "" {
		translated.Content = source.Content
	}
	return translated
}

func truncateProductText(source ProductText, maxChars int) ProductText {
	if maxChars <= 0 {
		return source
	}
	remaining := maxChars
	truncate := func(value string) string {
		if remaining <= 0 {
			return ""
		}
		count := utf8.RuneCountInString(value)
		if count <= remaining {
			remaining -= count
			return value
		}
		runes := []rune(value)
		value = string(runes[:remaining])
		remaining = 0
		return value
	}
	return ProductText{
		Title:       truncate(source.Title),
		Description: truncate(source.Description),
		Content:     truncate(source.Content),
	}
}
