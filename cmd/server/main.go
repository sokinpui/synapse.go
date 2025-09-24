package main

import (
	"fmt"
	"log"
	"net"

	"github.com/redis/go-redis/v9"
	pb "github.com/sokinpui/synapse.go/v2/grpc"
	"github.com/sokinpui/synapse.go/v2/internal/config"
	"github.com/sokinpui/synapse.go/v2/internal/server"
	"google.golang.org/grpc"
)

func main() {
	log.SetPrefix("server: ")

	cfg := config.Load()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	s := grpc.NewServer()
	pb.RegisterGenerateServer(s, server.New(redisClient))

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
