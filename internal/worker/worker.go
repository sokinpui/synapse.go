package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"synapse/internal/models"
	"synapse/internal/queue"
	"github.com/sokinpui/sllmi-go"
)

const sentinel = "[DONE]"

// GenAIWorker dequeues and processes generation tasks.
type GenAIWorker struct {
	workerID    string
	redisClient *redis.Client
	queue       *queue.RQueue
	llmRegistry *sllmi.Registry
}

// New creates a new GenAIWorker.
func New(redisClient *redis.Client, llmRegistry *sllmi.Registry) *GenAIWorker {
	return &GenAIWorker{
		workerID:    fmt.Sprintf("GenAIWorker-%d", os.Getpid()),
		redisClient: redisClient,
		queue:       queue.New(redisClient, "request_queue"),
		llmRegistry: llmRegistry,
	}
}

// Run starts the worker's main loop.
func (w *GenAIWorker) Run(ctx context.Context) {
	log.Printf("%s started.", w.workerID)
	for {
		select {
		case <-ctx.Done():
			log.Printf("%s shutting down.", w.workerID)
			return
		default:
			w.processNextTask(ctx)
		}
	}
}

func (w *GenAIWorker) processNextTask(ctx context.Context) {
	log.Println("Waiting for a generation task...")
	task, err := w.queue.Dequeue(ctx, 0) // 0 timeout means block indefinitely
	if err != nil {
		if err != redis.Nil {
			log.Printf("Failed to dequeue from Redis: %v. Retrying in 5 seconds.", err)
			time.Sleep(5 * time.Second)
		}
		return
	}
	if task == nil {
		return
	}

	log.Printf("Processing task: %s", task.TaskID)
	resultChannel := task.TaskID

	defer func() {
		if err := w.redisClient.Publish(ctx, resultChannel, sentinel).Err(); err != nil {
			log.Printf("Failed to publish sentinel for task %s: %v", task.TaskID, err)
		}
	}()

	model, err := w.llmRegistry.GetModel(task.ModelCode)
	if err != nil {
		log.Printf("Error getting model for task %s: %v", task.TaskID, err)
		errMsg := fmt.Sprintf("Error: %v", err)
		w.redisClient.Publish(ctx, resultChannel, errMsg)
		return
	}

	if task.Stream {
		err = w.processStream(ctx, task, model)
	} else {
		err = w.process(ctx, task, model)
	}

	if err != nil {
		log.Printf("Error processing generation task %s: %v", task.TaskID, err)
		errMsg := fmt.Sprintf("Error: %v", err)
		w.redisClient.Publish(ctx, resultChannel, errMsg)
	}
}

func (w *GenAIWorker) process(ctx context.Context, task *models.GenerationTask, model sllmi.LLM) error {
	result, err := model.Generate(ctx, task.Prompt, task.Config)
	if err != nil {
		return err
	}
	return w.redisClient.Publish(ctx, task.TaskID, result).Err()
}

func (w *GenAIWorker) processStream(ctx context.Context, task *models.GenerationTask, model sllmi.LLM) error {
	outCh, errCh := model.GenerateStream(ctx, task.Prompt, task.Config)

	for {
		select {
		case chunk, ok := <-outCh:
			if !ok {
				return nil // Stream finished
			}
			if err := w.redisClient.Publish(ctx, task.TaskID, chunk).Err(); err != nil {
				log.Printf("Failed to publish chunk for task %s: %v", task.TaskID, err)
			}
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
