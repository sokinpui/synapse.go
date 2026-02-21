package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sokinpui/synapse.go/internal/broker"
	"github.com/sokinpui/synapse.go/internal/color"
	"github.com/sokinpui/synapse.go/internal/models"
	"github.com/sokinpui/synapse.go/model"
)

type HTTPServer struct {
	broker      *broker.MemoryBroker
	llmRegistry *model.Registry
}

func NewHTTPServer(b *broker.MemoryBroker, llmRegistry *model.Registry) *HTTPServer {
	return &HTTPServer{
		broker:      b,
		llmRegistry: llmRegistry,
	}
}

func (s *HTTPServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /models", s.handleListModels)
	mux.HandleFunc("POST /generate", s.handleGenerate)

	// OpenAI Compatible API
	mux.HandleFunc("GET /v1/models", s.handleOpenAIListModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleOpenAIChatCompletions)
}

func (s *HTTPServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	modelCodes := s.llmRegistry.ListModels()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"models": modelCodes})
}

func (s *HTTPServer) handleOpenAIListModels(w http.ResponseWriter, r *http.Request) {
	modelCodes := s.llmRegistry.ListModels()
	now := time.Now().Unix()
	data := make([]models.OpenAIModel, len(modelCodes))
	for i, m := range modelCodes {
		data[i] = models.OpenAIModel{
			ID:      m,
			Object:  "model",
			Created: now,
			OwnedBy: "synapse",
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.OpenAIModelList{Object: "list", Data: data})
}

func (s *HTTPServer) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req models.GenerationTask
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	taskID := uuid.New().String()
	req.TaskID = taskID
	log.Printf("-> %s (HTTP), assigned task_id: %s", color.BlueString("Received request"), taskID)

	resCh := s.broker.Subscribe(taskID)
	defer s.broker.Unsubscribe(taskID)

	s.broker.Enqueue(&req)

	if req.Stream {
		s.streamHTTPResults(w, r, resCh)
		return
	}

	s.aggregateHTTPResults(w, resCh)
}

func (s *HTTPServer) handleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	var oaiReq models.OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&oaiReq); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	taskID := uuid.New().String()
	log.Printf("-> %s (OpenAI), assigned task_id: %s", color.BlueString("Received request"), taskID)

	var promptBuilder strings.Builder
	for _, msg := range oaiReq.Messages {
		promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	task := &models.GenerationTask{
		TaskID:    taskID,
		Prompt:    promptBuilder.String(),
		ModelCode: oaiReq.Model,
		Stream:    oaiReq.Stream,
		Config: &model.Config{
			Temperature:  oaiReq.Temperature,
			OutputLength: oaiReq.MaxTokens,
		},
	}

	resCh := s.broker.Subscribe(taskID)
	defer s.broker.Unsubscribe(taskID)
	s.broker.Enqueue(task)

	if task.Stream {
		s.streamOpenAIResults(w, r, task, resCh)
		return
	}
	s.aggregateOpenAIResults(w, task, resCh)
}

func (s *HTTPServer) streamHTTPResults(w http.ResponseWriter, r *http.Request, ch <-chan string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok || data == sentinel {
				return
			}
			jsonData, err := json.Marshal(map[string]string{"text": data})
			if err != nil {
				log.Printf("Error marshalling stream response: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

func (s *HTTPServer) aggregateHTTPResults(w http.ResponseWriter, ch <-chan string) {
	var sb strings.Builder
	for data := range ch {
		if data == sentinel {
			break
		}
		sb.WriteString(data)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"text": sb.String(),
	})
}

func (s *HTTPServer) streamOpenAIResults(w http.ResponseWriter, r *http.Request, task *models.GenerationTask, ch <-chan string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	now := time.Now().Unix()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok || data == sentinel {
				io.WriteString(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			chunk := models.ChatCompletionChunk{
				ID:      fmt.Sprintf("chatcmpl-%s", task.TaskID),
				Object:  "chat.completion.chunk",
				Created: now,
				Model:   task.ModelCode,
				Choices: []models.ChunkChoice{
					{
						Index: 0,
						Delta: models.OpenAIChatMessage{
							Content: data,
						},
						FinishReason: nil,
					},
				},
			}

			jsonData, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

func (s *HTTPServer) aggregateOpenAIResults(w http.ResponseWriter, task *models.GenerationTask, ch <-chan string) {
	var sb strings.Builder
	for data := range ch {
		if data == sentinel {
			break
		}
		sb.WriteString(data)
	}

	now := time.Now().Unix()

	resp := models.OpenAIChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", task.TaskID),
		Object:  "chat.completion",
		Created: now,
		Model:   task.ModelCode,
		Choices: []models.Choice{
			{
				Index: 0,
				Message: models.OpenAIChatMessage{
					Role:    "assistant",
					Content: sb.String(),
				},
				FinishReason: "stop",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
