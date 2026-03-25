package model

import (
	"context"
	"fmt"
	"io"
	"log"
	"github.com/sokinpui/synapse.go/internal/color"
	"os"

	openrouter "github.com/revrost/go-openrouter"
	"github.com/sokinpui/synapse.go/internal/config"
)

func init() {
	RegisterProvider(newOpenRouterProvider)
}

func newOpenRouterProvider(cfg *config.Config) (map[string]LLM, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	models := make(map[string]LLM)
	ctx := context.Background()

	keyCount := 0
	if apiKey != "" {
		keyCount = 1
	}
	log.Printf("Provider %s initialized with %d keys", color.GreenString("OpenRouter"), keyCount)

	for _, code := range cfg.Models.OpenRouter.Codes {
		model, err := NewOpenRouterModel(ctx, code, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini model '%s': %w", code, err)
		}
		models[code] = model
	}

	return models, nil
}

type OpenRouterModel struct {
	model  string
	apiKey string
}

func NewOpenRouterModel(ctx context.Context, modelCode string, apiKey string) (*OpenRouterModel, error) {
	return &OpenRouterModel{
		model:  modelCode,
		apiKey: apiKey,
	}, nil
}

func (orm *OpenRouterModel) Generate(ctx context.Context, prompt string, images [][]byte, config *Config) (*StreamChunk, error) {
	/* TODO: don't support Image yet */
	client := openrouter.NewClient(orm.apiKey)
	req := openrouter.ChatCompletionRequest{
		Model: orm.model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.UserMessage(prompt),
		},
	}

	if config != nil {
		if config.Temperature != nil {
			req.Temperature = *config.Temperature
		}
		if config.TopP != nil {
			req.TopP = *config.TopP
		}
		if config.TopK != nil {
			req.TopK = int(*config.TopK)
		}
		if config.OutputLength > 0 {
			req.MaxCompletionTokens = int(config.OutputLength)
		}
	}

	response, err := client.CreateChatCompletion(ctx, req)

	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}

	thought := ""
	if response.Choices[0].Message.ReasoningContent != nil {
		thought = *response.Choices[0].Message.ReasoningContent
	}

	return &StreamChunk{
		Text:    response.Choices[0].Message.Content.Text,
		Thought: thought,
	}, nil
}

func (orm *OpenRouterModel) GenerateStream(ctx context.Context, prompt string, images [][]byte, config *Config) (<-chan StreamChunk, <-chan error) {
	outCh := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		client := openrouter.NewClient(orm.apiKey)
		req := openrouter.ChatCompletionRequest{
			Model: orm.model,
			Messages: []openrouter.ChatCompletionMessage{
				openrouter.UserMessage(prompt),
			},
			Stream: true,
		}

		if config != nil {
			if config.Temperature != nil {
				req.Temperature = *config.Temperature
			}
			if config.TopP != nil {
				req.TopP = *config.TopP
			}
			if config.TopK != nil {
				req.TopK = int(*config.TopK)
			}
			if config.OutputLength > 0 {
				req.MaxCompletionTokens = int(config.OutputLength)
			}
		}
		stream, err := client.CreateChatCompletionStream(ctx, req)

		if err != nil && err != io.EOF {
			errCh <- fmt.Errorf("OpenRouter API error: %w", err)
			return
		}

		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err != nil {
				break
			}
			outCh <- StreamChunk{
				Text:    response.Choices[0].Delta.Content,
				Thought: response.Choices[0].Delta.ReasoningContent,
			}
		}
	}()

	return outCh, errCh
}

func (orm *OpenRouterModel) CountTokens(prompt string) (int, error) {
	/* 1 English character ≈ 0.3 token.
	1 Chinese character ≈ 0.6 token. */
	// loop via each char
	var tokenConnt float32 = 0.0
	for _, r := range prompt {
		if r <= 127 {
			// English char
			tokenConnt += 0.3
		} else {
			// Non-English char
			tokenConnt += 0.6
		}
	}
	return int(tokenConnt), nil

}
