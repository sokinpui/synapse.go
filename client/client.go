package client

import (
	"context"
	"io"

	pb "github.com/sokinpui/synapse.go/v2/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GenerationConfig struct {
	Temperature  *float32
	TopP         *float32
	TopK         *float32
	OutputLength *int32
}

type GenerateRequest struct {
	Prompt    string
	ModelCode string
	Stream    bool
	Config    *GenerationConfig
	Images    [][]byte
}

type Result struct {
	Text string
	Err  error
}

type Client interface {
	GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error)

	Close() error
}

type grpcClient struct {
	conn   *grpc.ClientConn
	client pb.GenerateClient
}

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

func (c *grpcClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *grpcClient) GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error) {
	pbReq := &pb.Request{
		Prompt:    req.Prompt,
		ModelCode: req.ModelCode,
		Stream:    req.Stream,
		Images:    req.Images,
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
