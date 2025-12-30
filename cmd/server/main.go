package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"runtime"
	"syscall"
	"time"

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

	llmRegistry, err := model.New(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize LLM registry: %v", err)
	}

	memBroker := broker.NewMemoryBroker(1000)

	concurrency := cfg.Worker.ConcurrencyMultiplier * runtime.NumCPU()
	w := worker.New(memBroker, llmRegistry, concurrency)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go w.Run(ctx)

	// gRPC Server
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		log.Fatalf("failed to listen gRPC: %v", err)
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterGenerateServer(grpcSrv, server.New(memBroker, llmRegistry))

	// HTTP Server
	mux := http.NewServeMux()
	httpSrv := server.NewHTTPServer(memBroker, llmRegistry)
	httpSrv.RegisterRoutes(mux)
	httpAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	hSrv := &http.Server{Addr: httpAddr, Handler: mux}

	log.Printf("gRPC Server listening at %v", grpcLis.Addr())
	log.Printf("HTTP Server listening at %s", httpAddr)

	go func() {
		if err := grpcSrv.Serve(grpcLis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	go func() {
		if err := hSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down servers...")

	grpcSrv.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(10*runtime.NumCPU())*time.Second)
	defer cancel()

	if err := hSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
}
