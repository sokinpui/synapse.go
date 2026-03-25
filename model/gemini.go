package model

import (
	"context"
	"fmt"
	"github.com/sokinpui/synapse.go/internal/config"
	"google.golang.org/genai"
	"google.golang.org/genai/tokenizer"
	"os"
	"strings"
)

func init() {
	RegisterProvider(newGeminiProvider)
}

func newGeminiProvider(cfg *config.Config) (map[string]LLM, error) {
	apiKeysVar := os.Getenv("GENAI_API_KEYS")
	apiKeys := strings.FieldsFunc(apiKeysVar, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})

	models := make(map[string]LLM)
	ctx := context.Background()
	balancer := NewKeyBalancer(apiKeys)

	for _, code := range cfg.Models.Gemini.Codes {
		model, err := NewGeminiModel(ctx, code, balancer)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini model '%s': %w", code, err)
		}
		models[code] = model
	}

	return models, nil
}

type GeminiModel struct {
	model    string
	balancer *KeyBalancer
}

func NewGeminiModel(ctx context.Context, modelCode string, balancer *KeyBalancer) (*GeminiModel, error) {
	return &GeminiModel{
		model:    modelCode,
		balancer: balancer,
	}, nil
}

// Generate performs a non-streaming text generation.
func (m *GeminiModel) Generate(ctx context.Context, prompt string, images [][]byte, config *Config) (*StreamChunk, error) {
	if m.balancer.KeyCount() == 0 {
		return nil, fmt.Errorf("%w: API key is required for generation", ErrConfiguration)
	}

	content, err := buildContent(prompt, images)
	if err != nil {
		return nil, err
	}

	genConfig := getGenConfig(m.model, config)
	var lastErr error

	for i := 0; i < m.balancer.KeyCount(); i++ {
		apiKey := m.balancer.PickKey()
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

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			return nil, fmt.Errorf("%w: no content in response", ErrGeneration)
		}

		res := &StreamChunk{}
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Thought {
				res.Thought += part.Text
				continue
			}
			res.Text += part.Text
		}
		return res, nil
	}

	return nil, fmt.Errorf("all API keys failed: %w", lastErr)
}

// GenerateStream performs a streaming text generation.
func (m *GeminiModel) GenerateStream(ctx context.Context, prompt string, images [][]byte, config *Config) (<-chan StreamChunk, <-chan error) {
	genConfig := getGenConfig(m.model, config)
	outCh := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		if m.balancer.KeyCount() == 0 {
			errCh <- fmt.Errorf("%w: API key is required for generation", ErrConfiguration)
			return
		}

		content, err := buildContent(prompt, images)
		if err != nil {
			errCh <- err
			return
		}

		var lastErr error

		for i := 0; i < m.balancer.KeyCount(); i++ {
			apiKey := m.balancer.PickKey()
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
					if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
						continue
					}
					for _, part := range resp.Candidates[0].Content.Parts {
						chunk := StreamChunk{}
						if part.Thought {
							chunk.Thought = part.Text
						} else {
							if part.Text == "" {
								continue
							}
							chunk.Text = part.Text
						}
						outCh <- chunk
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

func getGenConfig(modelCode string, config *Config) *genai.GenerateContentConfig {
	var thinking *genai.ThinkingConfig
	if strings.Contains(modelCode, "thinking") || strings.Contains(modelCode, "gemini-3") {
		thinking = &genai.ThinkingConfig{IncludeThoughts: true}
	}

	if config == nil {
		return &genai.GenerateContentConfig{ThinkingConfig: thinking}
	}

	return &genai.GenerateContentConfig{
		Temperature:     config.Temperature,
		TopP:            config.TopP,
		TopK:            config.TopK,
		MaxOutputTokens: config.OutputLength,
		ThinkingConfig:  thinking,
	}
}
