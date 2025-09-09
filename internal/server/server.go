package server

import (
	"context"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sllmi-go"
	pb "synapse/grpc"
	"synapse/internal/models"
	"synapse/internal/queue"
)

const sentinel = "[DONE]"

// Server implements the gRPC Generate service.
type Server struct {
	pb.UnimplementedGenerateServer
	redisClient *redis.Client
	queue       *queue.RQueue
}

// New creates a new gRPC server.
func New(redisClient *redis.Client) *Server {
	return &Server{
		redisClient: redisClient,
		queue:       queue.New(redisClient, "request_queue"),
	}
}

// GenerateTask handles a generation request.
func (s *Server) GenerateTask(req *pb.Request, stream pb.Generate_GenerateTaskServer) error {
	taskID := uuid.New().String()
	log.Printf("-> Received request, assigned task_id: %s", taskID)

	ctx := stream.Context()
	resultChannel := taskID

	pubsub := s.redisClient.Subscribe(ctx, resultChannel)
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Printf("Error subscribing to channel for task %s: %v", taskID, err)
		return status.Errorf(codes.Internal, "failed to subscribe to result channel")
	}

	task := s.createTask(taskID, req)

	if err := s.queue.Enqueue(ctx, task); err != nil {
		log.Printf("Error enqueuing task %s: %v", taskID, err)
		return status.Errorf(codes.Internal, "failed to enqueue task")
	}

	return s.streamResults(req, stream, pubsub.Channel())
}

func (s *Server) createTask(taskID string, req *pb.Request) *models.GenerationTask {
	var cfg *sllmigo.Config
	if req.Config != nil {
		cfg = &sllmigo.Config{
			Temperature:  req.Config.Temperature,
			TopP:         req.Config.TopP,
			TopK:         req.Config.TopK,
			OutputLength: req.Config.OutputLength,
		}
	}

	return &models.GenerationTask{
		TaskID:    taskID,
		Prompt:    req.Prompt,
		ModelCode: req.ModelCode,
		Stream:    req.Stream,
		Config:    cfg,
	}
}

func (s *Server) streamResults(req *pb.Request, stream pb.Generate_GenerateTaskServer, ch <-chan *redis.Message) error {
	var outputParts []string

	for msg := range ch {
		data := msg.Payload
		if data == sentinel {
			break
		}

		if req.Stream {
			if err := stream.Send(&pb.Response{OutputString: data}); err != nil {
				return err
			}
		} else {
			outputParts = append(outputParts, data)
		}
	}

	if !req.Stream {
		fullOutput := strings.Join(outputParts, "")
		if err := stream.Send(&pb.Response{OutputString: fullOutput}); err != nil {
			return err
		}
	}

	return nil
}
