package lmstudio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
	logger     *log.Logger
}

func NewClient(baseURL, authToken string, timeoutMinutes int, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Client{
		baseURL:   baseURL,
		authToken: authToken,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutMinutes) * time.Minute,
		},
		logger: logger,
	}
}

func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/chat", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("connecting to LM Studio: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &chatResp, nil
}

type StreamCallbacks struct {
	OnDelta    func(text string)
	OnToolCall func(event ToolCallEvent)
}

// ChatStream sends a streaming chat request via LM Studio's v1 SSE API.
// Calls OnDelta for message text chunks and OnToolCall for tool events.
// Falls back to regular JSON parsing when LM Studio doesn't return SSE.
func (c *Client) ChatStream(ctx context.Context, req *ChatRequest, cb StreamCallbacks) (*ChatResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/chat", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("connecting to LM Studio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, string(respBody))
	}

	ct := resp.Header.Get("Content-Type")
	c.logger.Printf("ChatStream Content-Type: %s", ct)

	if !strings.Contains(ct, "text/event-stream") {
		c.logger.Printf("ChatStream: non-SSE response, falling back to JSON parse")
		return c.parseJSONResponse(resp.Body)
	}

	return c.parseSSEStream(resp.Body, cb)
}

func (c *Client) parseJSONResponse(body io.Reader) (*ChatResponse, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	c.logger.Printf("ChatStream JSON body length: %d", len(data))

	var chatResp ChatResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}
	return &chatResp, nil
}

func (c *Client) parseSSEStream(body io.Reader, cb StreamCallbacks) (*ChatResponse, error) {
	var accumulated strings.Builder
	var finalResponse *ChatResponse
	var pendingTool ToolCallEvent
	eventCount := 0

	const maxBuf = 1024 * 1024 // 1 MB for large reasoning responses
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, maxBuf), maxBuf)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ": ") || strings.HasPrefix(line, "event: ") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			c.logger.Printf("ChatStream: failed to parse SSE data: %v (len=%d)", err, len(data))
			continue
		}
		eventCount++

		switch event.Type {
		case "message.delta":
			if event.Content != "" {
				accumulated.WriteString(event.Content)
				if cb.OnDelta != nil {
					cb.OnDelta(event.Content)
				}
			}

		case "tool_call.start":
			pendingTool = ToolCallEvent{Tool: event.Tool}

		case "tool_call.arguments":
			pendingTool.Arguments = event.Arguments

		case "tool_call.success":
			pendingTool.Output = event.Output
			pendingTool.Success = true
			if cb.OnToolCall != nil {
				cb.OnToolCall(pendingTool)
			}
			pendingTool = ToolCallEvent{}

		case "tool_call.failure":
			pendingTool.Success = false
			pendingTool.Reason = event.Content
			if cb.OnToolCall != nil {
				cb.OnToolCall(pendingTool)
			}
			pendingTool = ToolCallEvent{}

		case "chat.end":
			if event.Result != nil {
				finalResponse = event.Result
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", err)
	}

	c.logger.Printf("ChatStream: parsed %d events, finalResponse=%v", eventCount, finalResponse != nil)

	if finalResponse == nil {
		c.logger.Printf("ChatStream: no chat.end received, using accumulated text (%d bytes)", accumulated.Len())
		finalResponse = &ChatResponse{
			Output: []Output{{Type: "message", Content: accumulated.String()}},
		}
	}

	return finalResponse, nil
}

func (c *Client) ListModels(ctx context.Context) (*ModelsResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("connecting to LM Studio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &models, nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/models", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("connecting to LM Studio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}
