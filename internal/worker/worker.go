package worker

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/sokinpui/sllmi-go/v2"
	"github.com/sokinpui/synapse.go/internal/models"
	"github.com/sokinpui/synapse.go/internal/queue"
)

const sentinel = "[DONE]"

// GenAIWorker dequeues and processes generation tasks.
type GenAIWorker struct {
	workerID    string
	redisClient *redis.Client
	queue       *queue.RQueue
	llmRegistry *sllmi.Registry
}

func New(redisClient *redis.Client, llmRegistry *sllmi.Registry) *GenAIWorker {
	return &GenAIWorker{
		workerID:    fmt.Sprintf("GenAIWorker-%d", os.Getpid()),
		redisClient: redisClient,
		queue:       queue.New(redisClient, "request_queue"),
		llmRegistry: llmRegistry,
	}
}

func (w *GenAIWorker) Run(ctx context.Context) {
	log.Printf("%s started. Waiting for tasks...", w.workerID)

	taskCh := make(chan *models.GenerationTask)

	// Goroutine to fetch tasks from the queue
	go func() {
		defer close(taskCh)
		for {
			// Block indefinitely until a task is available or the context is canceled.
			task, err := w.queue.Dequeue(ctx, 0)
			if err != nil {
				// Context cancellation will cause Dequeue to return an error,
				// which is the expected way to stop this goroutine.
				if err != context.Canceled {
					log.Printf("Failed to dequeue from Redis: %v", err)
				}
				return
			}
			if task != nil {
				select {
				case taskCh <- task:
				case <-ctx.Done():
					return // Exit if context is canceled while waiting to send.
				}
			}
		}
	}()

	for {
		select {
		case task, ok := <-taskCh:
			if !ok {
				log.Printf("%s dequeue channel closed, shutting down.", w.workerID)
				return
			}
			w.processTask(ctx, task)
		case <-ctx.Done():
			log.Printf("%s shutting down.", w.workerID)
			return
		}
	}
}

func (w *GenAIWorker) processTask(ctx context.Context, task *models.GenerationTask) {
	log.Printf("-> Processing task: %s", task.TaskID)
	defer log.Printf("<- Finished task: %s", task.TaskID)

	taskCtx, cancelTask := context.WithCancel(ctx)
	defer cancelTask()

	go w.listenForCancellation(taskCtx, task.TaskID, cancelTask)

	resultChannel := task.TaskID

	defer func() {
		if err := w.redisClient.Publish(context.Background(), resultChannel, sentinel).Err(); err != nil {
			log.Printf("Failed to publish sentinel for task %s: %v", task.TaskID, err)
		}
	}()

	model, err := w.llmRegistry.GetModel(task.ModelCode)
	if err != nil {
		log.Printf("Error getting model for task %s: %v", task.TaskID, err)
		errMsg := fmt.Sprintf("Error: %v", err)
		w.redisClient.Publish(context.Background(), resultChannel, errMsg)
		return
	}

	if task.Stream {
		err = w.processStream(taskCtx, task, model)
	} else {
		err = w.process(taskCtx, task, model)
	}

	if err != nil {
		if err == context.Canceled {
			log.Printf("Task %s was canceled.", task.TaskID)
			return
		}
		log.Printf("Error processing generation task %s: %v", task.TaskID, err)
		errMsg := fmt.Sprintf("Error: %v", err)
		w.redisClient.Publish(context.Background(), resultChannel, errMsg)
	}
}

func (w *GenAIWorker) listenForCancellation(ctx context.Context, taskID string, cancel context.CancelFunc) {
	pubsub := w.redisClient.Subscribe(ctx, cancellationChannel(taskID))
	defer pubsub.Close()

	msg, err := pubsub.ReceiveMessage(ctx)
	if err != nil {
		// This is expected if the context is canceled (e.g., task completes normally).
		return
	}

	if msg != nil {
		log.Printf("Cancellation signal received for task %s. Canceling.", taskID)
		cancel()
	}
}

func (w *GenAIWorker) process(ctx context.Context, task *models.GenerationTask, model sllmi.LLM) error {
	result, err := model.Generate(ctx, task.Prompt, task.Images, task.Config)
	if err != nil {
		return err
	}
	return w.redisClient.Publish(ctx, task.TaskID, result).Err()
}

func (w *GenAIWorker) processStream(ctx context.Context, task *models.GenerationTask, model sllmi.LLM) error {
	outCh, errCh := model.GenerateStream(ctx, task.Prompt, task.Images, task.Config)

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

func cancellationChannel(taskID string) string {
	return "cancel:" + taskID
}
