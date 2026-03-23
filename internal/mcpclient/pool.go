package mcpclient

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

type conn struct {
	name    string
	cmd     *exec.Cmd
	stdin   *json.Encoder
	stdout  *bufio.Scanner
	tools   []ToolDef
	toolSet map[string]bool
	mu      sync.Mutex
	nextID  atomic.Int64
}

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type Pool struct {
	conns    map[string]*conn
	toolMap  map[string]string // tool name -> server name
	mu       sync.Mutex
	logger   *log.Logger
	command  map[string][]string
	env      map[string]map[string]string
}

func NewPool(logger *log.Logger) *Pool {
	if logger == nil {
		logger = log.New(os.Stderr, "mcpclient: ", log.LstdFlags)
	}
	return &Pool{
		conns:   make(map[string]*conn),
		toolMap: make(map[string]string),
		logger:  logger,
		command: make(map[string][]string),
		env:     make(map[string]map[string]string),
	}
}

func (p *Pool) Register(name string, command []string, env map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.command[name] = command
	p.env[name] = env
}

func (p *Pool) Connect(name string) error {
	p.mu.Lock()
	cmd, ok := p.command[name]
	envMap := p.env[name]
	p.mu.Unlock()

	if !ok {
		return fmt.Errorf("no command registered for %q", name)
	}

	return p.connectWithCommand(name, cmd, envMap)
}

func (p *Pool) connectWithCommand(name string, command []string, envMap map[string]string) error {
	if len(command) == 0 {
		return fmt.Errorf("empty command for %q", name)
	}

	cmd := exec.Command(command[0], command[1:]...)
	if len(envMap) > 0 {
		env := os.Environ()
		for k, v := range envMap {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe for %q: %w", name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe for %q: %w", name, err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %q: %w", name, err)
	}

	const maxBuf = 4 * 1024 * 1024
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, maxBuf), maxBuf)

	c := &conn{
		name:   name,
		cmd:    cmd,
		stdin:  json.NewEncoder(stdin),
		stdout: scanner,
	}

	if err := p.initialize(c); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("initializing %q: %w", name, err)
	}

	tools, err := p.listToolsConn(c)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("listing tools for %q: %w", name, err)
	}
	c.tools = tools
	c.toolSet = make(map[string]bool, len(tools))
	for _, t := range tools {
		c.toolSet[t.Name] = true
	}

	p.mu.Lock()
	if old, exists := p.conns[name]; exists {
		old.cmd.Process.Kill()
	}
	p.conns[name] = c
	for _, t := range tools {
		p.toolMap[t.Name] = name
	}
	p.mu.Unlock()

	p.logger.Printf("Connected %q with %d tools", name, len(tools))
	return nil
}

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (p *Pool) initialize(c *conn) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	if err := c.stdin.Encode(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]string{"name": "mcp-lmstudio", "version": "1.0.0"},
		},
	}); err != nil {
		return fmt.Errorf("sending initialize: %w", err)
	}

	if !c.stdout.Scan() {
		return fmt.Errorf("reading initialize response: %v", c.stdout.Err())
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("parsing initialize response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	if err := c.stdin.Encode(jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}); err != nil {
		return fmt.Errorf("sending initialized notification: %w", err)
	}

	return nil
}

func (p *Pool) listToolsConn(c *conn) ([]ToolDef, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	if err := c.stdin.Encode(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
	}); err != nil {
		return nil, fmt.Errorf("sending tools/list: %w", err)
	}

	if !c.stdout.Scan() {
		return nil, fmt.Errorf("reading tools/list response: %v", c.stdout.Err())
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parsing tools/list response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parsing tools list: %w", err)
	}
	return result.Tools, nil
}

func (p *Pool) CallTool(toolName string, args json.RawMessage) (string, error) {
	p.mu.Lock()
	serverName, ok := p.toolMap[toolName]
	if !ok {
		p.mu.Unlock()
		return "", fmt.Errorf("unknown tool %q", toolName)
	}
	c, exists := p.conns[serverName]
	p.mu.Unlock()

	if !exists {
		return "", fmt.Errorf("server %q not connected", serverName)
	}

	result, err := p.callToolConn(c, toolName, args)
	if err != nil {
		p.logger.Printf("Tool call %q failed, attempting reconnect: %v", toolName, err)
		if reconnErr := p.Connect(serverName); reconnErr != nil {
			return "", fmt.Errorf("tool call failed and reconnect failed: %w (original: %v)", reconnErr, err)
		}
		p.mu.Lock()
		c = p.conns[serverName]
		p.mu.Unlock()
		return p.callToolConn(c, toolName, args)
	}
	return result, nil
}

func (p *Pool) callToolConn(c *conn, toolName string, args json.RawMessage) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var parsedArgs interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &parsedArgs); err != nil {
			parsedArgs = map[string]interface{}{}
		}
	} else {
		parsedArgs = map[string]interface{}{}
	}

	id := c.nextID.Add(1)
	if err := c.stdin.Encode(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": parsedArgs,
		},
	}); err != nil {
		return "", fmt.Errorf("sending tools/call: %w", err)
	}

	if !c.stdout.Scan() {
		return "", fmt.Errorf("reading tools/call response: %v", c.stdout.Err())
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parsing tools/call response: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("tool %q error: %s", toolName, resp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return string(resp.Result), nil
	}

	var out string
	for _, c := range result.Content {
		if c.Type == "text" {
			out += c.Text
		}
	}
	if result.IsError {
		return out, fmt.Errorf("tool returned error: %s", out)
	}
	return out, nil
}

// ToolServerName returns which MCP server owns a given tool.
func (p *Pool) ToolServerName(toolName string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.toolMap[toolName]
}

// AllToolDefs returns all tools across all connected servers as OpenAI-compatible function tool definitions.
func (p *Pool) AllToolDefs() []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	var result []interface{}
	for _, c := range p.conns {
		for _, t := range c.tools {
			def := map[string]interface{}{
				"type": "function",
				"name": t.Name,
			}
			if t.Description != "" {
				def["description"] = t.Description
			}
			if len(t.Parameters) > 0 {
				var params interface{}
				if json.Unmarshal(t.Parameters, &params) == nil {
					def["parameters"] = params
				}
			}
			result = append(result, def)
		}
	}
	return result
}

// ConnectedServers returns the names of all connected MCP servers.
func (p *Pool) ConnectedServers() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var names []string
	for name := range p.conns {
		names = append(names, name)
	}
	return names
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, c := range p.conns {
		if c.cmd.Process != nil {
			c.cmd.Process.Kill()
		}
		p.logger.Printf("Closed MCP server %q", name)
	}
	p.conns = make(map[string]*conn)
	p.toolMap = make(map[string]string)
}
