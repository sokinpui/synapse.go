package model

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
	"google.golang.org/genai/tokenizer"
)

func init() {
	RegisterProvider(newGeminiProvider)
}

func newGeminiProvider() (map[string]LLM, error) {
	apiKeysVar := os.Getenv("GENAI_API_KEYS")
	var apiKeys []string
	if apiKeysVar != "" {
		for _, key := range strings.Split(apiKeysVar, ",") {
			if key != "" {
				apiKeys = append(apiKeys, key)
			}
		}
	}

	modelCodes := []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash-preview-09-2025",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite-preview-09-2025",
		"gemini-2.5-flash-lite",
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",
		"gemma-3-27b-it",
	}

	models := make(map[string]LLM)
	ctx := context.Background()

	for _, code := range modelCodes {
		model, err := NewGeminiModel(ctx, code, apiKeys)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini model '%s': %w", code, err)
		}
		models[code] = model
	}

	return models, nil
}

type GeminiModel struct {
	model   string
	apiKeys []string
}

func NewGeminiModel(ctx context.Context, modelCode string, apiKeys []string) (*GeminiModel, error) {
	return &GeminiModel{
		model:   modelCode,
		apiKeys: apiKeys,
	}, nil
}

func (m *GeminiModel) getShuffledKeys() []string {
	shuffledKeys := make([]string, len(m.apiKeys))
	copy(shuffledKeys, m.apiKeys)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(shuffledKeys), func(i, j int) { shuffledKeys[i], shuffledKeys[j] = shuffledKeys[j], shuffledKeys[i] })

	return shuffledKeys
}

// Generate performs a non-streaming text generation.
func (m *GeminiModel) Generate(ctx context.Context, prompt string, images [][]byte, config *Config) (string, error) {
	if len(m.apiKeys) == 0 {
		return "", fmt.Errorf("%w: API key is required for generation", ErrConfiguration)
	}

	content, err := buildContent(prompt, images)
	if err != nil {
		return "", err
	}

	genConfig := getGenConfig(config)
	var lastErr error

	for _, apiKey := range m.getShuffledKeys() {
		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
		if err != nil {
			lastErr = fmt.Errorf("failed to create genai client: %w", err)
			continue
		}

		resp, err := client.Models.GenerateContent(ctx, m.model, content, genConfig)
		if err != nil {
			lastErr = fmt.Errorf("%w: %v", ErrGeneration, err)
			continue
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("%w: no content in response", ErrGeneration)
		}

		return resp.Text(), nil
	}

	return "", fmt.Errorf("all API keys failed: %w", lastErr)
}

// GenerateStream performs a streaming text generation.
func (m *GeminiModel) GenerateStream(ctx context.Context, prompt string, images [][]byte, config *Config) (<-chan string, <-chan error) {
	genConfig := getGenConfig(config)
	outCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		if len(m.apiKeys) == 0 {
			errCh <- fmt.Errorf("%w: API key is required for generation", ErrConfiguration)
			return
		}

		content, err := buildContent(prompt, images)
		if err != nil {
			errCh <- err
			return
		}

		var lastErr error

		for _, apiKey := range m.getShuffledKeys() {
			client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
			if err != nil {
				lastErr = fmt.Errorf("failed to create genai client: %w", err)
				continue
			}

			streamErr := func() error {
				iter := client.Models.GenerateContentStream(ctx, m.model, content, genConfig)
				for resp, err := range iter {
					if err != nil {
						return err
					}
					if resp != nil && len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
						outCh <- resp.Text()
					}
				}
				return nil
			}()

			if streamErr != nil {
				lastErr = fmt.Errorf("%w: %v", ErrGeneration, streamErr)
				continue
			}
			return // Success
		}

		errCh <- fmt.Errorf("all API keys failed: %w", lastErr)
	}()
	return outCh, errCh
}

func buildContent(prompt string, images [][]byte) ([]*genai.Content, error) {
	parts := []*genai.Part{genai.NewPartFromText(prompt)}

	for _, imgBytes := range images {
		parts = append(parts, genai.NewPartFromBytes(imgBytes, "image/jpeg"))
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	return contents, nil
}

// CountTokens counts the number of tokens in a prompt.
func (m *GeminiModel) CountTokens(prompt string) (int, error) {
	tok, err := tokenizer.NewLocalTokenizer("gemini-2.5-flash")
	if err != nil {
		return 0, fmt.Errorf("token counting failed: %w", err)
	}

	ntoks, err := tok.CountTokens(genai.Text(prompt), nil)
	if err != nil {
		return 0, fmt.Errorf("token counting failed: %w", err)
	}

	return int(ntoks.TotalTokens), nil
}

func getGenConfig(config *Config) *genai.GenerateContentConfig {
	if config == nil {
		return &genai.GenerateContentConfig{}
	}

	var tools = []*genai.Tool{
		{
			GoogleSearch: &genai.GoogleSearch{},
			URLContext:   &genai.URLContext{},
		},
	}

	return &genai.GenerateContentConfig{
		Temperature:     config.Temperature,
		TopP:            config.TopP,
		TopK:            config.TopK,
		MaxOutputTokens: config.OutputLength,
		Tools:           tools,
	}
}
