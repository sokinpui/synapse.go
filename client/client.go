package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GenerationConfig struct {
	Temperature  *float32 `json:"temperature,omitempty"`
	TopP         *float32 `json:"top_p,omitempty"`
	TopK         *float32 `json:"top_k,omitempty"`
	OutputLength *int32   `json:"output_length,omitempty"`
}

type GenerateRequest struct {
	Prompt    string            `json:"prompt"`
	ModelCode string            `json:"model_code"`
	Stream    bool              `json:"stream"`
	Config    *GenerationConfig `json:"config,omitempty"`
	Images    [][]byte          `json:"images,omitempty"`
}

type Result struct {
	Text        string
	Thought     string
	Err         error
	IsKeepAlive bool
}

type Client interface {
	GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error)
	ListModels(ctx context.Context) ([]string, error)
	Close() error
}

type httpClient struct {
	baseURL    string
	httpClient *http.Client
}

func New(addr string) Client {
	if !strings.HasPrefix(addr, "http") {
		addr = "http://" + addr
	}
	return &httpClient{
		baseURL:    strings.TrimSuffix(addr, "/"),
		httpClient: &http.Client{},
	}
}

func (c *httpClient) Close() error {
	return nil
}

func (c *httpClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}

func (c *httpClient) GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/generate", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	resultChan := make(chan Result)
	if req.Stream {
		go c.handleStream(resp.Body, resultChan)
	} else {
		go c.handleUnary(resp.Body, resultChan)
	}

	return resultChan, nil
}

func (c *httpClient) handleUnary(body io.ReadCloser, ch chan<- Result) {
	defer body.Close()
	defer close(ch)

	var res struct {
		Text    string `json:"text"`
		Thought string `json:"thought"`
	}
	if err := json.NewDecoder(body).Decode(&res); err != nil {
		ch <- Result{Err: err}
		return
	}
	ch <- Result{Text: res.Text, Thought: res.Thought}
}

func (c *httpClient) handleStream(body io.ReadCloser, ch chan<- Result) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var res struct {
			Text    string `json:"text"`
			Thought string `json:"thought"`
		}
		if err := json.Unmarshal([]byte(data), &res); err != nil {
			continue
		}
		ch <- Result{Text: res.Text, Thought: res.Thought}
	}

	if err := scanner.Err(); err != nil {
		ch <- Result{Err: err}
	}
}
