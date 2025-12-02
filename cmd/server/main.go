package main

import (
	"fmt"
	"log"
	"net"

	"github.com/redis/go-redis/v9"
	pb "github.com/sokinpui/synapse.go/grpc"
	"github.com/sokinpui/synapse.go/internal/config"
	"github.com/sokinpui/synapse.go/internal/server"
	"github.com/sokinpui/synapse.go/model"
	"google.golang.org/grpc"
)

func main() {
	log.SetPrefix("server: ")

	cfg := config.Load()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	llmRegistry, err := model.New(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize LLM registry: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterGenerateServer(s, server.New(redisClient, llmRegistry))

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
