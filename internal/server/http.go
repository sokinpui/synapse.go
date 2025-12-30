package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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
}

func (s *HTTPServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	models := s.llmRegistry.ListModels()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"models": models})
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
			fmt.Fprintf(w, "data: %s\n\n", data)
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
