package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/infinitimeless/lmstudio-mcp/internal/artifacts"
	"github.com/infinitimeless/lmstudio-mcp/internal/chatlog"
	"github.com/infinitimeless/lmstudio-mcp/internal/config"
	"github.com/infinitimeless/lmstudio-mcp/internal/lmstudio"
	"github.com/infinitimeless/lmstudio-mcp/internal/mcpclient"
	"github.com/infinitimeless/lmstudio-mcp/internal/profile"
	"github.com/infinitimeless/lmstudio-mcp/internal/progress"
	"github.com/infinitimeless/lmstudio-mcp/internal/session"
	"github.com/infinitimeless/lmstudio-mcp/internal/taskgroup"
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
	Project       string   `json:"project,omitempty" jsonschema:"Project name to tag this session with for grouping related tasks"`
	GroupID       string
	GroupStep     int
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

type ListArtifactsArgs struct {
	SessionID  string `json:"session_id" jsonschema:"The session ID"`
	ToolFilter string `json:"tool_filter,omitempty" jsonschema:"Filter by tool name (e.g. read_file)"`
}

type GetArtifactArgs struct {
	SessionID string `json:"session_id" jsonschema:"The session ID"`
	Sequence  int    `json:"sequence,omitempty" jsonschema:"Artifact sequence number from list_session_artifacts"`
	FilePath  string `json:"file_path,omitempty" jsonschema:"File path to look up (for file-read artifacts)"`
}

type QueueItem struct {
	Task          string   `json:"task,omitempty" jsonschema:"Task description (for new task)"`
	Profile       string   `json:"profile,omitempty" jsonschema:"Agent profile key (e.g. coder, reviewer)"`
	Context       string   `json:"context,omitempty" jsonschema:"Context from a previous session"`
	MaxTokens     int      `json:"max_tokens,omitempty" jsonschema:"Token budget for this task"`
	Temperature   float64  `json:"temperature,omitempty" jsonschema:"Temperature override"`
	ContextLength int      `json:"context_length,omitempty" jsonschema:"Context window size in tokens"`
	Integrations  []string `json:"integrations,omitempty" jsonschema:"Integration keys to enable"`
	SystemPrompt  string   `json:"system_prompt,omitempty" jsonschema:"Override profile system prompt"`
	SessionID     string   `json:"session_id,omitempty" jsonschema:"Session ID to continue (makes this a continue_task instead of start_task)"`
	Message       string   `json:"message,omitempty" jsonschema:"Message for continue_task (used when session_id is set)"`
	Project       string   `json:"project,omitempty" jsonschema:"Project name override for this item (defaults to queue-level project)"`
}

type QueueTasksArgs struct {
	Tasks   []QueueItem `json:"tasks" jsonschema:"Ordered list of tasks to execute sequentially"`
	Project string      `json:"project,omitempty" jsonschema:"Project name applied to all tasks in this queue"`
}

type ChainItem struct {
	Task          string   `json:"task,omitempty" jsonschema:"Task description (for new task)"`
	Profile       string   `json:"profile,omitempty" jsonschema:"Agent profile key (e.g. coder, reviewer)"`
	Context       string   `json:"context,omitempty" jsonschema:"Additional context (chain results are injected automatically)"`
	MaxTokens     int      `json:"max_tokens,omitempty" jsonschema:"Token budget for this task"`
	Temperature   float64  `json:"temperature,omitempty" jsonschema:"Temperature override"`
	ContextLength int      `json:"context_length,omitempty" jsonschema:"Context window size in tokens"`
	Integrations  []string `json:"integrations,omitempty" jsonschema:"Integration keys to enable"`
	SystemPrompt  string   `json:"system_prompt,omitempty" jsonschema:"Override profile system prompt"`
	SessionID     string   `json:"session_id,omitempty" jsonschema:"Session ID to continue (makes this a continue_task)"`
	Message       string   `json:"message,omitempty" jsonschema:"Message for continue_task (used when session_id is set)"`
	Project       string   `json:"project,omitempty" jsonschema:"Project name override for this item (defaults to chain-level project)"`
}

type ChainTasksArgs struct {
	Tasks     []ChainItem `json:"tasks" jsonschema:"Ordered list of tasks forming the pipeline"`
	ChainMode string      `json:"chain_mode,omitempty" jsonschema:"How results flow forward: 'previous' (default, only last result) or 'all' (all accumulated results)"`
	Project   string      `json:"project,omitempty" jsonschema:"Project name applied to all tasks in this chain"`
}

type LoopTaskArgs struct {
	Directive     string   `json:"directive" jsonschema:"Primary directive prompt repeated each iteration"`
	Profile       string   `json:"profile,omitempty" jsonschema:"Agent profile key (e.g. coder, reviewer)"`
	MaxLoops      int      `json:"max_loops" jsonschema:"Maximum number of loop iterations"`
	StopPhrase    string   `json:"stop_phrase,omitempty" jsonschema:"If the worker response contains this phrase the loop stops early"`
	MaxTokens     int      `json:"max_tokens,omitempty" jsonschema:"Token budget per iteration"`
	Temperature   float64  `json:"temperature,omitempty" jsonschema:"Temperature override"`
	ContextLength int      `json:"context_length,omitempty" jsonschema:"Context window size in tokens"`
	Integrations  []string `json:"integrations,omitempty" jsonschema:"Integration keys to enable"`
	SystemPrompt  string   `json:"system_prompt,omitempty" jsonschema:"Override profile system prompt"`
	Project       string   `json:"project,omitempty" jsonschema:"Project name applied to all iterations"`
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

	lm := lmstudio.NewClient(cfg.APIBase, cfg.APIToken, cfg.RequestTimeout, logger)

	store, err := session.NewStore(cfg.SessionsDir)
	if err != nil {
		logger.Fatalf("Failed to init session store: %v", err)
	}
	sessions := session.NewManager(store, cfg.MaxSessionTokens, cfg.TokenWarningThreshold, cfg.TokenCriticalThreshold)
	prog := progress.NewManager(cfg.ProgressDir)
	profiles := profile.NewManager(cfg)

	chatWriter, err := chatlog.NewWriter(cfg.ChatlogDir)
	if err != nil {
		logger.Printf("Warning: chatlog disabled: %v", err)
	}

	artStore, err := artifacts.NewStore(cfg.ArtifactsDir)
	if err != nil {
		logger.Printf("Warning: artifact store disabled: %v", err)
	}

	groupStore, err := taskgroup.NewStore(cfg.SessionsDir)
	if err != nil {
		logger.Printf("Warning: task group store disabled: %v", err)
	}

	mcpPool := mcpclient.NewPool(logger)
	functionIntegrationSet := make(map[string]bool)
	for name, ic := range cfg.App.Integrations {
		if ic.Type == "function" && len(ic.Command) > 0 {
			mcpPool.Register(name, ic.Command, ic.Env)
			if err := mcpPool.Connect(name); err != nil {
				logger.Printf("Warning: failed to connect MCP server %q: %v", name, err)
			} else {
				functionIntegrationSet[name] = true
			}
		}
	}
	if len(functionIntegrationSet) > 0 {
		logger.Printf("Connected %d function integration(s): %v", len(functionIntegrationSet), mcpPool.ConnectedServers())
	}
	defer mcpPool.Close()

	hasFunctionIntegrations := func(keys []string) bool {
		for _, k := range keys {
			if functionIntegrationSet[k] {
				return true
			}
		}
		return false
	}

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

	tc := &taskContext{
		lm:                         lm,
		sessions:                   sessions,
		profiles:                   profiles,
		chatWriter:                 chatWriter,
		artStore:                   artStore,
		groups:                     groupStore,
		logger:                     logger,
		mcpPool:                    mcpPool,
		resolveSessionIntegrations: resolveSessionIntegrations,
		hasFunctionIntegrations:    hasFunctionIntegrations,
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

		r := tc.executeNewTask(ctx, args)
		if r.Error != "" {
			return errResult(r.Error), nil, nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Session: %s | Profile: %s | Model: %s\n\n", r.SessionID, r.Profile, r.Model)
		b.WriteString(r.Text)
		if r.Usage != nil {
			b.WriteString("\n\n")
			b.WriteString(sessions.FormatTokenUsage(r.Usage))
			if w := sessions.TokenWarning(r.Usage); w != "" {
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

		r := tc.executeContinueTask(ctx, args.SessionID, args.Message)
		if r.Error != "" {
			return errResult(r.Error), nil, nil
		}

		var b strings.Builder
		b.WriteString(r.Text)
		if r.Usage != nil {
			b.WriteString("\n\n")
			b.WriteString(sessions.FormatTokenUsage(r.Usage))
			if w := sessions.TokenWarning(r.Usage); w != "" {
				b.WriteString("\n" + w)
			}
		}
		return textResult(b.String()), nil, nil
	})

	// ── queue_tasks ────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "queue_tasks",
		Description: "Execute multiple independent tasks sequentially in one call. Each task can be a new task (provide task+profile) or a continuation (provide session_id+message). All results are collected and returned together, never truncated. Tasks do NOT share results with each other — use chain_tasks for that.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args QueueTasksArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("queue_tasks count=%d", len(args.Tasks))

		if len(args.Tasks) == 0 {
			return errResult("No tasks provided. Pass at least one task."), nil, nil
		}

		total := len(args.Tasks)

		var grp *taskgroup.Group
		if tc.groups != nil {
			var err error
			grp, err = tc.groups.Create(taskgroup.TypeQueue, total)
			if err != nil {
				logger.Printf("Warning: failed to create task group: %v", err)
			}
		}

		succeeded := 0
		failed := 0
		var out strings.Builder

		for i, item := range args.Tasks {
			step := i + 1
			isContinue := item.SessionID != ""

			if isContinue {
				fmt.Fprintf(&out, "=== Task %d/%d [CONTINUE %s] ===\n", step, total, item.SessionID)
			} else {
				label := item.Profile
				if label == "" {
					label = "default"
				}
				fmt.Fprintf(&out, "=== Task %d/%d [NEW - %s] ===\n", step, total, label)
			}

			if grp != nil {
				grp.CurrentStep = step
				tc.groups.Update(grp)
			}

			itemProject := item.Project
			if itemProject == "" {
				itemProject = args.Project
			}

			var r taskResult
			if isContinue {
				r = tc.executeContinueTask(ctx, item.SessionID, item.Message)
			} else {
				r = tc.executeNewTask(ctx, StartTaskArgs{
					Task:          item.Task,
					Profile:       item.Profile,
					Context:       item.Context,
					MaxTokens:     item.MaxTokens,
					Temperature:   item.Temperature,
					ContextLength: item.ContextLength,
					Integrations:  item.Integrations,
					SystemPrompt:  item.SystemPrompt,
					Project:       itemProject,
					GroupID:       grpID(grp),
					GroupStep:     step,
				})
			}

			if r.SessionID != "" && grp != nil {
				if latest, ok := tc.groups.Get(grp.ID); ok {
					grp = latest
				}
			}

			if chatWriter != nil && r.SessionID != "" {
				if step == 1 {
					chatWriter.WriteGroupStart(r.SessionID, grpID(grp), taskgroup.TypeQueue, total)
				}
				chatWriter.WriteGroupStep(r.SessionID, grpID(grp), step, total)
			}

			if r.Error != "" {
				failed++
				fmt.Fprintf(&out, "ERROR: %s\n", r.Error)
			} else {
				succeeded++
				if r.SessionID != "" {
					fmt.Fprintf(&out, "Session: %s", r.SessionID)
					if r.Model != "" {
						fmt.Fprintf(&out, " | Model: %s", r.Model)
					}
					out.WriteString("\n")
				}
				out.WriteString("\n")
				out.WriteString(r.Text)
				if r.Usage != nil {
					out.WriteString("\n\n")
					out.WriteString(sessions.FormatTokenUsage(r.Usage))
					if w := sessions.TokenWarning(r.Usage); w != "" {
						out.WriteString("\n" + w)
					}
				}
			}
			out.WriteString("\n\n")
		}

		if grp != nil {
			grp.Succeeded = succeeded
			grp.Failed = failed
			if failed > 0 && succeeded == 0 {
				grp.Status = taskgroup.StatusFailed
			} else {
				grp.Status = taskgroup.StatusCompleted
			}
			tc.groups.Update(grp)
		}

		lastSessionID := ""
		if grp != nil && len(grp.SessionIDs) > 0 {
			lastSessionID = grp.SessionIDs[len(grp.SessionIDs)-1]
		}
		if chatWriter != nil && lastSessionID != "" {
			chatWriter.WriteGroupComplete(lastSessionID, grpID(grp), taskgroup.TypeQueue, succeeded, failed, false)
		}

		fmt.Fprintf(&out, "=== Queue Complete: %d/%d succeeded", succeeded, total)
		if failed > 0 {
			fmt.Fprintf(&out, " | %d failed", failed)
		}
		out.WriteString(" ===\n")

		return textResult(out.String()), nil, nil
	})

	// ── chain_tasks ────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "chain_tasks",
		Description: "Execute tasks sequentially as a pipeline where results flow forward. Each task receives previous results as context. Set chain_mode to 'previous' (default, only last result) or 'all' (all accumulated results). Each item can be a new task (task+profile) or continuation (session_id+message). Stops on first failure since downstream steps depend on prior results.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ChainTasksArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("chain_tasks count=%d mode=%s", len(args.Tasks), args.ChainMode)

		if len(args.Tasks) < 2 {
			return errResult("Chain requires at least 2 tasks."), nil, nil
		}

		mode := args.ChainMode
		if mode == "" {
			mode = "previous"
		}
		if mode != "previous" && mode != "all" {
			return errResult("chain_mode must be 'previous' or 'all'."), nil, nil
		}

		total := len(args.Tasks)

		var grp *taskgroup.Group
		if tc.groups != nil {
			var err error
			grp, err = tc.groups.Create(taskgroup.TypeChain, total)
			if err != nil {
				logger.Printf("Warning: failed to create task group: %v", err)
			} else {
				grp.ChainMode = mode
				tc.groups.Update(grp)
			}
		}

		completed := 0
		failed := 0
		var out strings.Builder
		var allResults []chainStepResult

		for i, item := range args.Tasks {
			step := i + 1
			isContinue := item.SessionID != ""

			chainCtx := buildChainContext(allResults, mode, i)

			contextNote := ""
			if i > 0 {
				if mode == "previous" {
					contextNote = fmt.Sprintf(" (received: step %d result)", i)
				} else {
					contextNote = fmt.Sprintf(" (received: steps 1-%d results)", i)
				}
			}

			if isContinue {
				fmt.Fprintf(&out, "=== Step %d/%d [CONTINUE %s]%s ===\n", step, total, item.SessionID, contextNote)
			} else {
				label := item.Profile
				if label == "" {
					label = "default"
				}
				fmt.Fprintf(&out, "=== Step %d/%d [NEW - %s]%s ===\n", step, total, label, contextNote)
			}

			if grp != nil {
				grp.CurrentStep = step
				tc.groups.Update(grp)
			}

			itemProject := item.Project
			if itemProject == "" {
				itemProject = args.Project
			}

			var r taskResult
			if isContinue {
				msg := item.Message
				if chainCtx != "" {
					msg = "--- RESULTS FROM PREVIOUS CHAIN STEPS ---\n" + chainCtx + "\n--- YOUR TASK ---\n" + msg
				}
				r = tc.executeContinueTask(ctx, item.SessionID, msg)
			} else {
				taskCtx := item.Context
				if chainCtx != "" {
					if taskCtx != "" {
						taskCtx = taskCtx + "\n\n" + chainCtx
					} else {
						taskCtx = chainCtx
					}
				}
				r = tc.executeNewTask(ctx, StartTaskArgs{
					Task:          item.Task,
					Profile:       item.Profile,
					Context:       taskCtx,
					MaxTokens:     item.MaxTokens,
					Temperature:   item.Temperature,
					ContextLength: item.ContextLength,
					Integrations:  item.Integrations,
					SystemPrompt:  item.SystemPrompt,
					Project:       itemProject,
					GroupID:       grpID(grp),
					GroupStep:     step,
				})
			}

			if r.SessionID != "" && grp != nil {
				if latest, ok := tc.groups.Get(grp.ID); ok {
					grp = latest
				}
			}

			if chatWriter != nil && r.SessionID != "" {
				if step == 1 {
					chatWriter.WriteGroupStart(r.SessionID, grpID(grp), taskgroup.TypeChain, total)
				}
				chatWriter.WriteGroupStep(r.SessionID, grpID(grp), step, total)
			}

			if r.Error != "" {
				failed++
				fmt.Fprintf(&out, "ERROR: %s\n\n", r.Error)

				if grp != nil {
					grp.Succeeded = completed
					grp.Failed = 1
					grp.Status = taskgroup.StatusFailed
					tc.groups.Update(grp)
				}
				if chatWriter != nil && r.SessionID != "" {
					chatWriter.WriteGroupComplete(r.SessionID, grpID(grp), taskgroup.TypeChain, completed, 1, false)
				}

				fmt.Fprintf(&out, "=== Chain Stopped: step %d/%d failed | Mode: %s ===\n", step, total, mode)
				return textResult(out.String()), nil, nil
			}

			completed++
			allResults = append(allResults, chainStepResult{
				StepNum:   step,
				Profile:   r.Profile,
				SessionID: r.SessionID,
				Text:      r.Text,
			})

			if r.SessionID != "" {
				fmt.Fprintf(&out, "Session: %s", r.SessionID)
				if r.Model != "" {
					fmt.Fprintf(&out, " | Model: %s", r.Model)
				}
				out.WriteString("\n")
			}
			out.WriteString("\n")
			out.WriteString(r.Text)
			if r.Usage != nil {
				out.WriteString("\n\n")
				out.WriteString(sessions.FormatTokenUsage(r.Usage))
				if w := sessions.TokenWarning(r.Usage); w != "" {
					out.WriteString("\n" + w)
				}
			}
			out.WriteString("\n\n")
		}

		if grp != nil {
			grp.Succeeded = completed
			grp.Status = taskgroup.StatusCompleted
			tc.groups.Update(grp)
		}

		lastSessionID := ""
		if grp != nil && len(grp.SessionIDs) > 0 {
			lastSessionID = grp.SessionIDs[len(grp.SessionIDs)-1]
		}
		if chatWriter != nil && lastSessionID != "" {
			chatWriter.WriteGroupComplete(lastSessionID, grpID(grp), taskgroup.TypeChain, completed, 0, false)
		}

		fmt.Fprintf(&out, "=== Chain Complete: %d/%d succeeded | Mode: %s ===\n", completed, total, mode)
		return textResult(out.String()), nil, nil
	})

	// ── loop_task ──────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "loop_task",
		Description: "Execute a task iteratively with fresh context each loop. The directive prompt is repeated every iteration, with the previous iteration's result passed as context. Loops until max_loops or until stop_phrase is detected in the worker response. Each iteration gets a fresh session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LoopTaskArgs) (*mcp.CallToolResult, any, error) {
		logger.Printf("loop_task max=%d profile=%s directive=%s", args.MaxLoops, args.Profile, truncate(args.Directive, 80))

		if args.Directive == "" {
			return errResult("directive is required."), nil, nil
		}
		if args.MaxLoops < 1 {
			return errResult("max_loops must be at least 1."), nil, nil
		}

		var grp *taskgroup.Group
		if tc.groups != nil {
			var err error
			grp, err = tc.groups.Create(taskgroup.TypeLoop, args.MaxLoops)
			if err != nil {
				logger.Printf("Warning: failed to create task group: %v", err)
			} else {
				grp.Directive = truncate(args.Directive, 200)
				grp.StopPhrase = args.StopPhrase
				tc.groups.Update(grp)
			}
		}

		var out strings.Builder
		var prevResult string
		stoppedEarly := false
		completed := 0

		for i := 0; i < args.MaxLoops; i++ {
			step := i + 1
			taskInput := args.Directive
			var taskCtx string
			if prevResult != "" {
				taskCtx = fmt.Sprintf("--- PREVIOUS ITERATION RESULT (iteration %d) ---\n%s", i, prevResult)
			}

			if grp != nil {
				grp.CurrentStep = step
				tc.groups.Update(grp)
			}

			r := tc.executeNewTask(ctx, StartTaskArgs{
				Task:          taskInput,
				Profile:       args.Profile,
				Context:       taskCtx,
				MaxTokens:     args.MaxTokens,
				Temperature:   args.Temperature,
				ContextLength: args.ContextLength,
				Integrations:  args.Integrations,
				SystemPrompt:  args.SystemPrompt,
				Project:       args.Project,
				GroupID:       grpID(grp),
				GroupStep:     step,
			})

			if r.SessionID != "" && grp != nil {
				if latest, ok := tc.groups.Get(grp.ID); ok {
					grp = latest
				}
			}

			if chatWriter != nil && r.SessionID != "" {
				if step == 1 {
					chatWriter.WriteGroupStart(r.SessionID, grpID(grp), taskgroup.TypeLoop, args.MaxLoops)
				}
				chatWriter.WriteGroupStep(r.SessionID, grpID(grp), step, args.MaxLoops)
			}

			if r.Error != "" {
				if grp != nil {
					grp.Succeeded = completed
					grp.Failed = 1
					grp.Status = taskgroup.StatusFailed
					tc.groups.Update(grp)
				}
				if chatWriter != nil && r.SessionID != "" {
					chatWriter.WriteGroupComplete(r.SessionID, grpID(grp), taskgroup.TypeLoop, completed, 1, false)
				}
				fmt.Fprintf(&out, "=== Iteration %d/%d [ERROR] ===\n", step, args.MaxLoops)
				fmt.Fprintf(&out, "ERROR: %s\n\n", r.Error)
				fmt.Fprintf(&out, "=== Loop Stopped: iteration %d/%d failed ===\n", step, args.MaxLoops)
				return textResult(out.String()), nil, nil
			}

			completed++
			prevResult = r.Text

			stopped := ""
			if args.StopPhrase != "" && strings.Contains(r.Text, args.StopPhrase) {
				stoppedEarly = true
				stopped = " [STOPPED - stop phrase detected]"
			}

			fmt.Fprintf(&out, "=== Iteration %d/%d%s ===\n", step, args.MaxLoops, stopped)
			if r.SessionID != "" {
				fmt.Fprintf(&out, "Session: %s\n", r.SessionID)
			}
			out.WriteString("\n")
			out.WriteString(r.Text)
			if r.Usage != nil {
				out.WriteString("\n\n")
				out.WriteString(sessions.FormatTokenUsage(r.Usage))
				if w := sessions.TokenWarning(r.Usage); w != "" {
					out.WriteString("\n" + w)
				}
			}
			out.WriteString("\n\n")

			if stoppedEarly {
				break
			}
		}

		if grp != nil {
			grp.Succeeded = completed
			grp.StoppedEarly = stoppedEarly
			grp.Status = taskgroup.StatusCompleted
			tc.groups.Update(grp)
		}

		lastSessionID := ""
		if grp != nil && len(grp.SessionIDs) > 0 {
			lastSessionID = grp.SessionIDs[len(grp.SessionIDs)-1]
		}
		if chatWriter != nil && lastSessionID != "" {
			chatWriter.WriteGroupComplete(lastSessionID, grpID(grp), taskgroup.TypeLoop, completed, 0, stoppedEarly)
		}

		fmt.Fprintf(&out, "=== Loop Complete: %d/%d iterations", completed, args.MaxLoops)
		if stoppedEarly {
			out.WriteString(" | Stopped early: yes")
		}
		out.WriteString(" ===\n")

		return textResult(out.String()), nil, nil
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

		var summary string
		if sess.UsesToolProxy {
			respReq := &lmstudio.ResponsesRequest{
				Model: sess.Model,
				Input: progress.SaveProgressPrompt,
			}
			if sess.LatestResponseID != "" {
				respReq.PreviousResponseID = sess.LatestResponseID
			}
			respResult, err := lm.Responses(ctx, respReq)
			if err != nil {
				return errResult(fmt.Sprintf("Error getting summary: %v", err)), nil, nil
			}
			if respResult.Usage != nil {
				sessions.AddTokens(sess.ID, respResult.Usage.InputTokens, respResult.Usage.OutputTokens, respResult.ID)
			}
			for _, out := range respResult.Output {
				if t := out.MessageText(); t != "" {
					summary = t
				}
			}
		} else {
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
			summary = extractMessages(chatResp.Output)
		}

		sess, _ = sessions.Get(args.SessionID)

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
			var summary string
			if sess.UsesToolProxy {
				respReq := &lmstudio.ResponsesRequest{
					Model: sess.Model,
					Input: progress.SaveProgressPrompt,
				}
				if sess.LatestResponseID != "" {
					respReq.PreviousResponseID = sess.LatestResponseID
				}
				respResult, err := lm.Responses(ctx, respReq)
				if err != nil {
					logger.Printf("Error getting summary: %v", err)
				} else {
					if respResult.Usage != nil {
						sessions.AddTokens(sess.ID, respResult.Usage.InputTokens, respResult.Usage.OutputTokens, respResult.ID)
					}
					for _, out := range respResult.Output {
						if t := out.MessageText(); t != "" {
							summary = t
						}
					}
				}
			} else {
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
					summary = extractMessages(chatResp.Output)
				}
			}

			if summary != "" {
				sess, _ = sessions.Get(args.SessionID)
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

	// ── list_session_artifacts ──────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_session_artifacts",
		Description: "List tool results (file reads, searches, etc.) captured during a session. Returns metadata only — use get_session_artifact to retrieve contents.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListArtifactsArgs) (*mcp.CallToolResult, any, error) {
		if artStore == nil {
			return errResult("Artifact store not available."), nil, nil
		}

		arts, err := artStore.List(args.SessionID)
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}
		if len(arts) == 0 {
			return textResult("No artifacts captured for this session."), nil, nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Artifacts for %s:\n", args.SessionID)
		for _, a := range arts {
			if args.ToolFilter != "" && a.Tool != args.ToolFilter {
				continue
			}
			fmt.Fprintf(&b, "  [%d] %s", a.Sequence, a.Tool)
			if a.FilePath != "" {
				fmt.Fprintf(&b, "  path=%s", a.FilePath)
			}
			fmt.Fprintf(&b, "  size=%d bytes", a.ContentSize)
			fmt.Fprintf(&b, "  %s\n", a.Timestamp.Format("15:04:05"))
		}
		return textResult(b.String()), nil, nil
	})

	// ── get_session_artifact ────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_session_artifact",
		Description: "Retrieve the full content of a tool result captured during a session. Look up by sequence number or file path. Use list_session_artifacts first to see what's available.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetArtifactArgs) (*mcp.CallToolResult, any, error) {
		if artStore == nil {
			return errResult("Artifact store not available."), nil, nil
		}

		var art *artifacts.Artifact
		var content string

		if args.FilePath != "" {
			art, content, err = artStore.GetByPath(args.SessionID, args.FilePath)
		} else {
			art, content, err = artStore.Get(args.SessionID, args.Sequence)
		}
		if err != nil {
			return errResult(fmt.Sprintf("Error: %v", err)), nil, nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Artifact [%d] %s", art.Sequence, art.Tool)
		if art.FilePath != "" {
			fmt.Fprintf(&b, " — %s", art.FilePath)
		}
		fmt.Fprintf(&b, " (%d bytes)\n\n", art.ContentSize)
		b.WriteString(content)
		return textResult(b.String()), nil, nil
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

// isFileTruncated detects when LM Studio has truncated a read_file tool
// result. LM Studio silently truncates tool outputs at ~50K characters and
// appends '... (truncated)' inside the JSON text value.
func isFileTruncated(tool, output string) bool {
	if tool != "read_file" {
		return false
	}
	trimmed := strings.TrimRight(output, " \t\n\r")
	// The output is JSON like [{"type":"text","text":"...content... (truncated)"}]
	// Check both raw suffix and JSON-wrapped suffix.
	return strings.HasSuffix(trimmed, "... (truncated)") ||
		strings.HasSuffix(trimmed, "... (truncated)\"}]") ||
		strings.HasSuffix(trimmed, "... (truncated)\"}")
}

// rereadFile extracts the file path from tool arguments and reads the full
// file directly from disk, bypassing LM Studio's truncation.
func rereadFile(args json.RawMessage) (string, error) {
	var parsed struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if parsed.Path == "" {
		return "", fmt.Errorf("no path in arguments")
	}
	data, err := os.ReadFile(parsed.Path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", parsed.Path, err)
	}
	return string(data), nil
}

func buildStreamCallbacks(cw *chatlog.Writer, as *artifacts.Store, logger *log.Logger, sessionID string) lmstudio.StreamCallbacks {
	var lastStatusProgress float64
	return lmstudio.StreamCallbacks{
		OnDelta: func(delta string) {
			if cw != nil {
				cw.WriteDelta(sessionID, delta)
			}
		},
		OnReasoning: func(phase, text string) {
			if cw == nil {
				return
			}
			switch phase {
			case "start":
				cw.WriteReasoningStart(sessionID)
			case "delta":
				cw.WriteReasoningDelta(sessionID, text)
			case "end":
				cw.WriteReasoningEnd(sessionID)
			}
		},
		OnToolCallStart: func(tool string) {
			if cw != nil {
				cw.WriteToolCallStart(sessionID, tool)
			}
		},
		OnToolCallResult: func(tc lmstudio.ToolCallEvent) {
			if cw != nil {
				args := ""
				if tc.Arguments != nil {
					args = string(tc.Arguments)
				}
				output := tc.Output
				if len(output) > 4096 {
					output = output[:4096] + "\n... (truncated, full content in artifacts)"
				}
				cw.WriteToolCallResult(sessionID, tc.Tool, args, output, tc.Reason, tc.Success)
			}
			if as != nil && tc.Success && tc.Output != "" {
				artifactContent := tc.Output
				if isFileTruncated(tc.Tool, tc.Output) {
					if full, err := rereadFile(tc.Arguments); err == nil {
						artifactContent = full
						logger.Printf("Re-read full file for artifact (%d bytes, was %d truncated)", len(full), len(tc.Output))
					} else {
						logger.Printf("Warning: truncation detected but re-read failed: %v", err)
					}
				}
				if err := as.Store(sessionID, tc.Tool, tc.Arguments, artifactContent, nil); err != nil {
					logger.Printf("Warning: failed to store artifact for %s/%s: %v", sessionID, tc.Tool, err)
				}
			}
		},
		OnStatus: func(phase string, progress float64) {
			if cw == nil {
				return
			}
			if progress-lastStatusProgress < 0.05 && progress < 1 {
				return
			}
			lastStatusProgress = progress
			cw.WriteStatus(sessionID, phase, progress)
		},
		OnError: func(errType, message string) {
			if cw != nil {
				cw.WriteError(sessionID, errType+": "+message)
			}
		},
	}
}

