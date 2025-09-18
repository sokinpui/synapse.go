package client

import (
	"context"
	"io"

	pb "github.com/sokinpui/synapse.go/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GenerationConfig mirrors the protobuf message for convenience.
type GenerationConfig struct {
	Temperature  *float32
	TopP         *float32
	TopK         *int32
	OutputLength *int32
}

// GenerateRequest holds parameters for a generation request.
type GenerateRequest struct {
	Prompt    string
	ModelCode string
	Stream    bool
	Config    *GenerationConfig
}

// Result holds either a text chunk from the stream or an error.
type Result struct {
	Text string
	Err  error
}

// Client is an interface for interacting with the Synapse generation service.
type Client interface {
	// GenerateTask sends a prompt for generation and returns a channel for streaming results.
	// If an error occurs during the stream, it will be sent on the channel.
	// The channel is closed once the stream is complete.
	// An error is returned if the initial request fails.
	GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error)

	// Close closes the connection to the server.
	Close() error
}

// grpcClient is the gRPC implementation of the Client interface.
type grpcClient struct {
	conn   *grpc.ClientConn
	client pb.GenerateClient
}

// New creates a new Synapse client connected to the given address.
func New(addr string) (Client, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &grpcClient{
		conn:   conn,
		client: pb.NewGenerateClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *grpcClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// GenerateTask implements the Client interface.
func (c *grpcClient) GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error) {
	pbReq := &pb.Request{
		Prompt:    req.Prompt,
		ModelCode: req.ModelCode,
		Stream:    req.Stream,
	}

	if req.Config != nil {
		pbReq.Config = &pb.GenerationConfig{
			Temperature:  req.Config.Temperature,
			TopP:         req.Config.TopP,
			TopK:         req.Config.TopK,
			OutputLength: req.Config.OutputLength,
		}
	}

	stream, err := c.client.GenerateTask(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan Result)

	go func() {
		defer close(resultChan)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return // Stream finished successfully
			}
			if err != nil {
				resultChan <- Result{Err: err}
				return
			}
			resultChan <- Result{Text: resp.GetOutputString()}
		}
	}()

	return resultChan, nil
}
