package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/signal"
	"runtime"
	"syscall"

	pb "github.com/sokinpui/synapse.go/grpc"
	"github.com/sokinpui/synapse.go/internal/broker"
	"github.com/sokinpui/synapse.go/internal/config"
	"github.com/sokinpui/synapse.go/internal/server"
	"github.com/sokinpui/synapse.go/internal/worker"
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

	llmRegistry, err := model.New(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize LLM registry: %v", err)
	}

	memBroker := broker.NewMemoryBroker(1000)

	concurrency := cfg.Worker.ConcurrencyMultiplier * runtime.NumCPU()
	w := worker.New(memBroker, llmRegistry, concurrency)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := grpc.NewServer()
	pb.RegisterGenerateServer(s, server.New(memBroker, llmRegistry))

	go w.Run(ctx)

	log.Printf("Server listening at %v", lis.Addr())
	go func() {
		<-ctx.Done()
		s.GracefulStop()
	}()

	if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
		log.Fatalf("failed to serve: %v", err)
	}
}
