package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("MCP LM Studio Orchestrator - Test Client")
	fmt.Println("==================================================")
	fmt.Println("NOTE: LM Studio must be running on http://127.0.0.1:1234")
	fmt.Println("      with a model loaded for tests to pass.")
	fmt.Println("==================================================")

	cmd := exec.Command("./mcp-lmstudio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	defer cmd.Process.Kill()

	reader := bufio.NewReader(stdout)
	testsFailed := false

	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Printf("SENT: %s\n", string(b))
		stdin.Write(append(b, '\n'))
	}

	receive := func() map[string]any {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Fatal(err)
		}
		fmt.Printf("RECV: %s\n", string(line))
		var res map[string]any
		json.Unmarshal(line, &res)
		return res
	}

	waitFor := func(id float64) map[string]any {
		for {
			resp := receive()
			if resp == nil {
				return nil
			}
			if resp["id"] == id {
				return resp
			}
		}
	}

	checkError := func(resp map[string]any, testName string) bool {
		if errObj, hasError := resp["error"]; hasError {
			fmt.Printf("\n  FAILED: %s returned error:\n", testName)
			errorJSON, _ := json.MarshalIndent(errObj, "", "  ")
			fmt.Println(string(errorJSON))
			testsFailed = true
			return true
		}
		return false
	}

	checkResult := func(resp map[string]any, testName string) bool {
		result, hasResult := resp["result"].(map[string]any)
		if !hasResult {
			fmt.Printf("\n  FAILED: %s has no result\n", testName)
			testsFailed = true
			return false
		}
		content, hasContent := result["content"].([]any)
		if !hasContent || len(content) == 0 {
			fmt.Printf("\n  FAILED: %s has no content\n", testName)
			testsFailed = true
			return false
		}
		textContent, ok := content[0].(map[string]any)
		if !ok {
			fmt.Printf("\n  FAILED: %s content format invalid\n", testName)
			testsFailed = true
			return false
		}
		text, ok := textContent["text"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			fmt.Printf("\n  FAILED: %s empty text\n", testName)
			testsFailed = true
			return false
		}
		return true
	}

	runTest := func(id float64, name string, params map[string]any) {
		fmt.Printf("\n--- Test: %s ---\n", name)
		send(map[string]any{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params":  params,
			"id":      id,
		})
		resp := waitFor(id)
		if resp == nil {
			fmt.Printf("  FAILED: %s no response\n", name)
			testsFailed = true
			return
		}
		if checkError(resp, name) {
			return
		}
		if checkResult(resp, name) {
			fmt.Printf("  PASSED: %s\n", name)
			result, _ := json.MarshalIndent(resp["result"], "", "  ")
			fmt.Println(string(result))
		}
	}

	// Initialize
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test-client", "version": "1.0.0"},
		},
		"id": 1,
	})
	waitFor(1)

	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})

	// Test: health_check
	runTest(2, "health_check", map[string]any{
		"name":      "health_check",
		"arguments": map[string]any{},
	})

	// Test: list_models
	runTest(3, "list_models", map[string]any{
		"name":      "list_models",
		"arguments": map[string]any{},
	})

	// Test: list_profiles
	runTest(4, "list_profiles", map[string]any{
		"name":      "list_profiles",
		"arguments": map[string]any{},
	})

	// Test: list_integrations
	runTest(5, "list_integrations", map[string]any{
		"name":      "list_integrations",
		"arguments": map[string]any{},
	})

	// Test: list_sessions (should be empty)
	runTest(6, "list_sessions", map[string]any{
		"name":      "list_sessions",
		"arguments": map[string]any{},
	})

	// Test: start_task
	fmt.Println("\n--- Test: start_task ---")
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "start_task",
			"arguments": map[string]any{
				"task":    "What is 2+2? Answer in one sentence.",
				"profile": "coder",
			},
		},
		"id": 7,
	})
	resp := waitFor(7)
	if resp != nil && !checkError(resp, "start_task") && checkResult(resp, "start_task") {
		fmt.Println("  PASSED: start_task")
		result, _ := json.MarshalIndent(resp["result"], "", "  ")
		fmt.Println(string(result))

		// Extract session_id from response text for follow-up tests
		if r, ok := resp["result"].(map[string]any); ok {
			if content, ok := r["content"].([]any); ok && len(content) > 0 {
				if tc, ok := content[0].(map[string]any); ok {
					text := tc["text"].(string)
					if strings.HasPrefix(text, "Session: sess_") {
						parts := strings.Fields(text)
						if len(parts) >= 2 {
							sessionID := parts[1]

							// Test: get_session_status
							runTest(8, "get_session_status", map[string]any{
								"name":      "get_session_status",
								"arguments": map[string]any{"session_id": sessionID},
							})

							// Test: list_sessions (should have one now)
							runTest(9, "list_sessions", map[string]any{
								"name":      "list_sessions",
								"arguments": map[string]any{},
							})

							// Test: end_session
							runTest(10, "end_session", map[string]any{
								"name":      "end_session",
								"arguments": map[string]any{"session_id": sessionID},
							})
						}
					}
				}
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	if testsFailed {
		fmt.Println("SOME TESTS FAILED")
		fmt.Println(strings.Repeat("=", 50))
		os.Exit(1)
	} else {
		fmt.Println("ALL TESTS PASSED")
		fmt.Println(strings.Repeat("=", 50))
	}
}
