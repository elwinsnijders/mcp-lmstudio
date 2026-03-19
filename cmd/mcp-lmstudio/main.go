package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/infinitimeless/lmstudio-mcp/internal/config"
	"github.com/infinitimeless/lmstudio-mcp/internal/lmstudio"
	"github.com/infinitimeless/lmstudio-mcp/internal/profile"
	"github.com/infinitimeless/lmstudio-mcp/internal/progress"
	"github.com/infinitimeless/lmstudio-mcp/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "lmstudio-bridge"
	ServerVersion = "1.0.0"
)

type EmptyArgs struct{}

type StartTaskArgs struct {
	Task          string   `json:"task" jsonschema:"The task description for the worker AI"`
	Profile       string   `json:"profile,omitempty" jsonschema:"Agent profile key (e.g. coder, reviewer, tester, researcher, debugger)"`
	Context       string   `json:"context,omitempty" jsonschema:"Context from a previous session such as progress file content"`
	MaxTokens     int      `json:"max_tokens,omitempty" jsonschema:"Token budget for this session (default 175000)"`
	Temperature   float64  `json:"temperature,omitempty" jsonschema:"Temperature override (0 uses profile default)"`
	ContextLength int      `json:"context_length,omitempty" jsonschema:"Context window size in tokens"`
	Integrations  []string `json:"integrations,omitempty" jsonschema:"Integration keys to enable (e.g. filesystem or playwright)"`
	SystemPrompt  string   `json:"system_prompt,omitempty" jsonschema:"Override profile system prompt (shared prompt still applies)"`
}

type ContinueTaskArgs struct {
	SessionID string `json:"session_id" jsonschema:"The session ID to continue"`
	Message   string `json:"message" jsonschema:"Message to send to the worker AI"`
}

type SaveProgressArgs struct {
	SessionID string `json:"session_id" jsonschema:"The session ID to save progress for"`
	Notes     string `json:"notes,omitempty" jsonschema:"Orchestrator notes to include in the progress file"`
}

type LoadProgressArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID to load progress for"`
	FilePath  string `json:"file_path,omitempty" jsonschema:"Direct path to a progress file"`
}

type SessionIDArgs struct {
	SessionID string `json:"session_id" jsonschema:"The session ID"`
}

type EndSessionArgs struct {
	SessionID string `json:"session_id" jsonschema:"The session ID to end"`
	Save      bool   `json:"save,omitempty" jsonschema:"Save progress before ending the session"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logFile, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file %s: %v", cfg.LogFile, err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "MCP: ", log.LstdFlags)

	lm := lmstudio.NewClient(cfg.APIBase, cfg.APIToken, cfg.RequestTimeout)

	store, err := session.NewStore(cfg.SessionsDir)
	if err != nil {
		logger.Fatalf("Failed to init session store: %v", err)
	}
	sessions := session.NewManager(store, cfg.MaxSessionTokens, cfg.TokenWarningThreshold, cfg.TokenCriticalThreshold)
	prog := progress.NewManager(cfg.ProgressDir)
	profiles := profile.NewManager(cfg)

	resolveSessionIntegrations := func(sess *session.Session) []interface{} {
		if len(sess.IntegrationKeys) == 0 {
			return nil
		}
		ints, err := profiles.ResolveIntegrations(sess.IntegrationKeys)
		if err != nil {
			logger.Printf("Warning: could not resolve integrations for session %s: %v", sess.ID, err)
			return nil
		}
		return ints
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: ServerName, Version: ServerVersion},
		&mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{
				Tools: &mcp.ToolCapabilities{ListChanged: true},
			},
		},
	)

	// ── health_check ────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "health_check",
		Description: "Check if LM Studio API is accessible.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		logger.Println("health_check")
		if err := lm.HealthCheck(ctx); err != nil {
			return errResult(fmt.Sprintf("LM Studio unreachable: %v", err)), nil, nil
		}
		return textResult("LM Studio API is running and accessible."), nil, nil
	})

	// ── list_models ─────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_models",
		Description: "List all available models in LM Studio.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		logger.Println("list_models")
		resp, err := lm.ListModels(ctx)
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}
		if len(resp.Models) == 0 {
			return textResult("No models found."), nil, nil
		}
		var b strings.Builder
		for _, m := range resp.Models {
			fmt.Fprintf(&b, "- %s (%s)", m.DisplayName, m.Key)
			if len(m.LoadedInstances) > 0 {
				fmt.Fprintf(&b, " [loaded, ctx:%d]", m.LoadedInstances[0].Config.ContextLength)
			}
			if m.Capabilities != nil && m.Capabilities.TrainedForToolUse {
				b.WriteString(" [tools]")
			}
			b.WriteString("\n")
		}
		return textResult(b.String()), nil, nil
	})

	// ── list_profiles ───────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_profiles",
		Description: "List available agent profiles (coder, reviewer, tester, etc). Use the key with start_task.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		profs := profiles.ListProfiles()
		if len(profs) == 0 {
			return textResult("No profiles configured. Add profiles to config.json."), nil, nil
		}
		var b strings.Builder
		for key, p := range profs {
			fmt.Fprintf(&b, "- %s: \"%s\" -- %s\n  temp=%.1f integrations=[%s]\n",
				key, p.Label, p.Description, p.Temperature, strings.Join(p.Integrations, ", "))
		}
		return textResult(b.String()), nil, nil
	})

	// ── list_integrations ───────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_integrations",
		Description: "List available integrations that can be passed to start_task.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		ints := profiles.ListIntegrations()
		if len(ints) == 0 {
			return textResult("No integrations configured. Add integrations to config.json."), nil, nil
		}
		var b strings.Builder
		for key, i := range ints {
			fmt.Fprintf(&b, "- %s: \"%s\" (%s) -- %s\n", key, i.Label, i.Type, i.Description)
		}
		return textResult(b.String()), nil, nil
	})

	// ── start_task ──────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "start_task",
		Description: "Start a new worker AI session. Pass task + profile key. All prompts and integrations resolve from config.json. Returns session_id for continue_task.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args StartTaskArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("start_task profile=%s task=%s", args.Profile, truncate(args.Task, 80))

		integrations, err := profiles.ResolveProfileIntegrations(args.Profile, args.Integrations)
		if err != nil {
			return errResult(fmt.Sprintf("Integration error: %v", err)), nil, nil
		}

		sampling := profiles.ResolveSampling(args.Profile, args.Temperature)
		ctxLen := profiles.ResolveContextLength(args.Profile, args.ContextLength)

		model := profiles.ResolveModel(args.Profile)

		sess, err := sessions.Create(args.Task, args.Profile, model, args.MaxTokens, args.Integrations)
		if err != nil {
			return errResult(fmt.Sprintf("Session creation error: %v", err)), nil, nil
		}

		systemPrompt := profiles.AssembleSystemPrompt(args.Profile, args.SystemPrompt, args.Context, 0, sess.TokensMax)

		chatReq := &lmstudio.ChatRequest{
			Model:           model,
			Input:           args.Task,
			SystemPrompt:    systemPrompt,
			Temperature:     sampling.Temperature,
			TopP:            sampling.TopP,
			TopK:            sampling.TopK,
			MinP:            sampling.MinP,
			RepeatPenalty:   sampling.RepeatPenalty,
			MaxOutputTokens: sampling.MaxOutputTokens,
			Reasoning:       sampling.Reasoning,
			ContextLength:   ctxLen,
		}
		if len(integrations) > 0 {
			chatReq.Integrations = integrations
		}

		chatResp, err := lm.Chat(ctx, chatReq)
		if err != nil {
			return errResult(fmt.Sprintf("Session %s created but LM Studio error: %v", sess.ID, err)), nil, nil
		}

		_, usage, err := sessions.AddTokens(sess.ID, chatResp.Stats.InputTokens, chatResp.Stats.TotalOutputTokens, chatResp.ResponseID)
		if err != nil {
			logger.Printf("Token tracking error: %v", err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Session: %s | Profile: %s | Model: %s\n\n", sess.ID, args.Profile, chatResp.ModelInstanceID)
		b.WriteString(formatOutput(chatResp.Output))
		if usage != nil {
			b.WriteString("\n\n")
			b.WriteString(sessions.FormatTokenUsage(usage))
			if w := sessions.TokenWarning(usage); w != "" {
				b.WriteString("\n" + w)
			}
		}
		return textResult(b.String()), nil, nil
	})

	// ── continue_task ───────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "continue_task",
		Description: "Continue an existing worker AI session. Worker remembers the full conversation via stateful chat.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ContinueTaskArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("continue_task session=%s", args.SessionID)

		sess, err := sessions.Get(args.SessionID)
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}
		if sess.Status != session.StatusActive {
			return errResult(fmt.Sprintf("Session %s is %s, not active.", sess.ID, sess.Status)), nil, nil
		}

		chatReq := &lmstudio.ChatRequest{
			Model: sess.Model,
			Input: args.Message,
		}
		if ints := resolveSessionIntegrations(sess); len(ints) > 0 {
			chatReq.Integrations = ints
		}
		if sess.LatestResponseID != "" {
			chatReq.PreviousResponseID = sess.LatestResponseID
		}

		chatResp, err := lm.Chat(ctx, chatReq)
		if err != nil {
			return errResult(fmt.Sprintf("LM Studio error: %v", err)), nil, nil
		}

		_, usage, err := sessions.AddTokens(sess.ID, chatResp.Stats.InputTokens, chatResp.Stats.TotalOutputTokens, chatResp.ResponseID)
		if err != nil {
			logger.Printf("Token tracking error: %v", err)
		}

		var b strings.Builder
		b.WriteString(formatOutput(chatResp.Output))
		if usage != nil {
			b.WriteString("\n\n")
			b.WriteString(sessions.FormatTokenUsage(usage))
			if w := sessions.TokenWarning(usage); w != "" {
				b.WriteString("\n" + w)
			}
		}
		return textResult(b.String()), nil, nil
	})

	// ── save_progress ───────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_progress",
		Description: "Ask the worker to summarize progress then save to a markdown file. Use load_progress to retrieve it for a new session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SaveProgressArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("save_progress session=%s", args.SessionID)

		sess, err := sessions.Get(args.SessionID)
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}

		chatReq := &lmstudio.ChatRequest{
			Model: sess.Model,
			Input: progress.SaveProgressPrompt,
		}
		if ints := resolveSessionIntegrations(sess); len(ints) > 0 {
			chatReq.Integrations = ints
		}
		if sess.LatestResponseID != "" {
			chatReq.PreviousResponseID = sess.LatestResponseID
		}

		chatResp, err := lm.Chat(ctx, chatReq)
		if err != nil {
			return errResult(fmt.Sprintf("Error getting summary: %v", err)), nil, nil
		}

		sessions.AddTokens(sess.ID, chatResp.Stats.InputTokens, chatResp.Stats.TotalOutputTokens, chatResp.ResponseID)
		sess, _ = sessions.Get(args.SessionID)

		summary := extractMessages(chatResp.Output)
		path, err := prog.Save(&progress.Info{
			SessionID:    sess.ID,
			Task:         sess.Task,
			Profile:      sess.Profile,
			Model:        sess.Model,
			TokensUsed:   sess.TokensUsed,
			TokensMax:    sess.TokensMax,
			StartedAt:    sess.CreatedAt,
			LastActiveAt: sess.LastActiveAt,
			Summary:      summary,
			Notes:        args.Notes,
		})
		if err != nil {
			return errResult(fmt.Sprintf("Error writing progress: %v", err)), nil, nil
		}

		return textResult(fmt.Sprintf("Progress saved to: %s\n\n%s", path, summary)), nil, nil
	})

	// ── load_progress ───────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "load_progress",
		Description: "Load a progress file from a previous session. Pass the content as context to start_task.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LoadProgressArgs) (*mcp.CallToolResult, any, error) {
		var content string
		var err error

		if args.FilePath != "" {
			content, err = prog.LoadFile(args.FilePath)
		} else if args.SessionID != "" {
			content, err = prog.Load(args.SessionID)
		} else {
			return errResult("Provide either session_id or file_path."), nil, nil
		}
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}
		return textResult(content), nil, nil
	})

	// ── get_session_status ──────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_session_status",
		Description: "Get token usage, status, and metadata for a session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SessionIDArgs) (*mcp.CallToolResult, any, error) {
		sess, err := sessions.Get(args.SessionID)
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}

		pct := float64(sess.TokensUsed) / float64(sess.TokensMax) * 100
		var b strings.Builder
		fmt.Fprintf(&b, "Session: %s\n", sess.ID)
		fmt.Fprintf(&b, "Status: %s\n", sess.Status)
		fmt.Fprintf(&b, "Profile: %s\n", sess.Profile)
		fmt.Fprintf(&b, "Task: %s\n", truncate(sess.Task, 120))
		fmt.Fprintf(&b, "Tokens: %d / %d (%.1f%%)\n", sess.TokensUsed, sess.TokensMax, pct)
		fmt.Fprintf(&b, "Exchanges: %d\n", len(sess.ResponseIDs))
		fmt.Fprintf(&b, "Created: %s\n", sess.CreatedAt.Format(time.RFC3339))
		fmt.Fprintf(&b, "Last Active: %s\n", sess.LastActiveAt.Format(time.RFC3339))
		if len(sess.IntegrationKeys) > 0 {
			fmt.Fprintf(&b, "Integrations: %s\n", strings.Join(sess.IntegrationKeys, ", "))
		}
		return textResult(b.String()), nil, nil
	})

	// ── list_sessions ───────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "List all tracked sessions with status and token usage.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		all := sessions.List()
		if len(all) == 0 {
			return textResult("No sessions."), nil, nil
		}
		var b strings.Builder
		for _, s := range all {
			pct := float64(s.TokensUsed) / float64(s.TokensMax) * 100
			fmt.Fprintf(&b, "- %s [%s] profile=%s tokens=%d/%d (%.0f%%) task=%s\n",
				s.ID, s.Status, s.Profile, s.TokensUsed, s.TokensMax, pct, truncate(s.Task, 60))
		}
		return textResult(b.String()), nil, nil
	})

	// ── end_session ─────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "end_session",
		Description: "End a session. Set save=true to save progress first.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EndSessionArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("end_session session=%s save=%v", args.SessionID, args.Save)

		sess, err := sessions.Get(args.SessionID)
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}

		var savedPath string
		if args.Save {
			chatReq := &lmstudio.ChatRequest{
				Model: sess.Model,
				Input: progress.SaveProgressPrompt,
			}
			if ints := resolveSessionIntegrations(sess); len(ints) > 0 {
				chatReq.Integrations = ints
			}
			if sess.LatestResponseID != "" {
				chatReq.PreviousResponseID = sess.LatestResponseID
			}

			chatResp, err := lm.Chat(ctx, chatReq)
			if err != nil {
				logger.Printf("Error getting summary: %v", err)
			} else {
				sessions.AddTokens(sess.ID, chatResp.Stats.InputTokens, chatResp.Stats.TotalOutputTokens, chatResp.ResponseID)
				sess, _ = sessions.Get(args.SessionID)

				summary := extractMessages(chatResp.Output)
				savedPath, err = prog.Save(&progress.Info{
					SessionID:    sess.ID,
					Task:         sess.Task,
					Profile:      sess.Profile,
					Model:        sess.Model,
					TokensUsed:   sess.TokensUsed,
					TokensMax:    sess.TokensMax,
					StartedAt:    sess.CreatedAt,
					LastActiveAt: sess.LastActiveAt,
					Summary:      summary,
				})
				if err != nil {
					logger.Printf("Error saving progress: %v", err)
				}
			}
		}

		if err := sessions.UpdateStatus(args.SessionID, session.StatusCompleted); err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}

		msg := fmt.Sprintf("Session %s ended.", args.SessionID)
		if savedPath != "" {
			msg += fmt.Sprintf(" Progress saved to: %s", savedPath)
		}
		return textResult(msg), nil, nil
	})

	// ── Start server ────────────────────────────────────────────────────

	transport := &mcp.LoggingTransport{
		Transport: &mcp.StdioTransport{},
		Writer:    logFile,
	}

	mcpSession, err := server.Connect(context.Background(), transport, nil)
	if err != nil {
		logger.Fatalf("Connection error: %v", err)
	}

	if err := mcpSession.Wait(); err != nil {
		logger.Printf("Session closed: %v", err)
	}
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func errResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}
}

func formatOutput(items []lmstudio.Output) string {
	var last string
	for _, item := range items {
		if item.Type == "message" && item.Content != "" {
			last = item.Content
		}
	}
	return last
}

func extractMessages(items []lmstudio.Output) string {
	var last string
	for _, item := range items {
		if item.Type == "message" && item.Content != "" {
			last = item.Content
		}
	}
	return last
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
