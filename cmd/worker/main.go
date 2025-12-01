package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/sokinpui/synapse.go/v2/internal/config"
	"github.com/sokinpui/synapse.go/v2/internal/worker"
	"github.com/sokinpui/synapse.go/v2/model"
)

func main() {
	log.SetPrefix("worker: ")

	cfg := config.Load()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	llmRegistry, err := model.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize LLM registry: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutdown signal received, stopping worker...")
		cancel()
	}()

	concurrency := cfg.Worker.ConcurrencyMultiplier * runtime.NumCPU()
	w := worker.New(redisClient, llmRegistry, concurrency)
	w.Run(ctx)
}
