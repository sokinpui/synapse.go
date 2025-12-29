package server

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/sokinpui/synapse.go/grpc"
	"github.com/sokinpui/synapse.go/internal/broker"
	"github.com/sokinpui/synapse.go/internal/color"
	"github.com/sokinpui/synapse.go/internal/models"
	"github.com/sokinpui/synapse.go/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sentinel = "[DONE]"

type Server struct {
	pb.UnimplementedGenerateServer
	broker      *broker.MemoryBroker
	llmRegistry *model.Registry
}

func New(b *broker.MemoryBroker, llmRegistry *model.Registry) *Server {
	return &Server{
		broker:      b,
		llmRegistry: llmRegistry,
	}
}

func (s *Server) ListModels(ctx context.Context, req *pb.ListModelsRequest) (*pb.ListModelsResponse, error) {
	if s.llmRegistry == nil {
		return nil, status.Error(codes.Internal, "LLM registry not initialized on server")
	}
	models := s.llmRegistry.ListModels()
	return &pb.ListModelsResponse{Models: models}, nil
}

func (s *Server) GenerateTask(req *pb.Request, stream pb.Generate_GenerateTaskServer) error {
	taskID := uuid.New().String()
	log.Printf("-> %s, assigned task_id: %s", color.BlueString("Received request"), taskID)

	doneChan := make(chan struct{})
	defer close(doneChan)

	defer log.Printf("<- %s for task_id: %s", color.GreenString("Finished request"), taskID)

	ctx := stream.Context()
	go s.handleCancellation(ctx, taskID, doneChan)

	resultChannel := taskID

	resCh := s.broker.Subscribe(resultChannel)
	defer s.broker.Unsubscribe(resultChannel)

	keepAliveCtx, cancelKeepAlive := context.WithCancel(ctx)
	defer cancelKeepAlive()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := stream.Send(&pb.Response{Type: pb.Response_KEEPALIVE}); err != nil {
					log.Printf("Error sending keep-alive for task %s: %v", taskID, err)
					return
				}
			case <-keepAliveCtx.Done():
				return
			}
		}
	}()

	task := s.createTask(taskID, req)

	s.broker.Enqueue(task)

	return s.streamResults(req, stream, resCh, cancelKeepAlive)
}

func (s *Server) handleCancellation(ctx context.Context, taskID string, doneChan <-chan struct{}) {
	select {
	case <-doneChan:
		return
	case <-ctx.Done():
		log.Printf("Client cancelled request for task %s. Publishing cancellation.", taskID)
		s.broker.SignalCancel(taskID)
	}
}

func (s *Server) createTask(taskID string, req *pb.Request) *models.GenerationTask {
	var cfg *model.Config
	if req.Config != nil {
		cfg = &model.Config{
			Temperature: req.Config.Temperature,
			TopP:        req.Config.TopP,
			TopK:        req.Config.TopK,
		}
		if req.Config.OutputLength != nil {
			cfg.OutputLength = *req.Config.OutputLength
		}
	}

	return &models.GenerationTask{
		TaskID:    taskID,
		Prompt:    req.Prompt,
		ModelCode: req.ModelCode,
		Stream:    req.Stream,
		Config:    cfg,
		Images:    req.Images,
	}
}

func (s *Server) streamResults(req *pb.Request, stream pb.Generate_GenerateTaskServer, ch <-chan string, cancelKeepAlive context.CancelFunc) error {
	var outputParts []string
	firstMessageReceived := false

	for data := range ch {
		if !firstMessageReceived {
			cancelKeepAlive()
			firstMessageReceived = true
		}

		if data == sentinel {
			break
		}

		if req.Stream {
			if err := stream.Send(&pb.Response{Type: pb.Response_CHUNK, Chunk: data}); err != nil {
				return err
			}
		} else {
			outputParts = append(outputParts, data)
		}
	}

	if !req.Stream {
		fullOutput := strings.Join(outputParts, "")
		if err := stream.Send(&pb.Response{Type: pb.Response_CHUNK, Chunk: fullOutput}); err != nil {
			return err
		}
	}

	return nil
}
