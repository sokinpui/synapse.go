package model

import (
	"context"
	"fmt"
	"sync"

	"github.com/sokinpui/synapse.go/internal/config"
)

type StreamChunk struct {
	Text    string `json:"text,omitempty"`
	Thought string `json:"thought,omitempty"`
}

type LLM interface {
	Generate(ctx context.Context, prompt string, images [][]byte, config *Config) (*StreamChunk, error)
	GenerateStream(ctx context.Context, prompt string, images [][]byte, config *Config) (<-chan StreamChunk, <-chan error)
	CountTokens(prompt string) (int, error)
}

type ModelProvider func(cfg *config.Config) (map[string]LLM, error)

var providers []ModelProvider

func RegisterProvider(provider ModelProvider) {
	providers = append(providers, provider)
}

type Registry struct {
	models map[string]LLM
	mu     sync.RWMutex
}

func New(cfg *config.Config) (*Registry, error) {
	r := &Registry{}
	if err := r.Reload(cfg); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) Reload(cfg *config.Config) error {
	allModels := make(map[string]LLM)
	for _, provider := range providers {
		providerModels, err := provider(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize a model provider: %w", err)
		}
		for name, model := range providerModels {
			if _, exists := allModels[name]; exists {
				fmt.Printf("Warning: Model '%s' is being overwritten by a new provider.\n", name)
			}
			allModels[name] = model
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.models = allModels
	return nil
}

func (r *Registry) GetModel(modelCode string) (LLM, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, ok := r.models[modelCode]
	if ok {
		return model, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelCode)
}

func (r *Registry) ListModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.models))
	for k := range r.models {
		keys = append(keys, k)
	}
	return keys
}
