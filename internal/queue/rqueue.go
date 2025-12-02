package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sokinpui/synapse.go/internal/models"
)

type RQueue struct {
	redisClient *redis.Client
	name        string
}

func New(redisClient *redis.Client, name string) *RQueue {
	return &RQueue{
		redisClient: redisClient,
		name:        name,
	}
}

func (q *RQueue) Enqueue(ctx context.Context, task *models.GenerationTask) error {
	item, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return q.redisClient.LPush(ctx, q.name, item).Err()
}

func (q *RQueue) Dequeue(ctx context.Context, timeout time.Duration) (*models.GenerationTask, error) {
	data, err := q.redisClient.BRPop(ctx, timeout, q.name).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	if len(data) < 2 {
		return nil, nil
	}

	var task models.GenerationTask
	if err := json.Unmarshal([]byte(data[1]), &task); err != nil {
		return nil, err
	}
	return &task, nil
}
