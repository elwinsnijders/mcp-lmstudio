package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/infinitimeless/lmstudio-mcp/internal/artifacts"
	"github.com/infinitimeless/lmstudio-mcp/internal/chatlog"
	"github.com/infinitimeless/lmstudio-mcp/internal/lmstudio"
	"github.com/infinitimeless/lmstudio-mcp/internal/mcpclient"
	"github.com/infinitimeless/lmstudio-mcp/internal/profile"
	"github.com/infinitimeless/lmstudio-mcp/internal/session"
	"github.com/infinitimeless/lmstudio-mcp/internal/taskgroup"
)

type taskContext struct {
	lm                         *lmstudio.Client
	sessions                   *session.Manager
	profiles                   *profile.Manager
	chatWriter                 *chatlog.Writer
	artStore                   *artifacts.Store
	groups                     *taskgroup.Store
	logger                     *log.Logger
	mcpPool                    *mcpclient.Pool
	resolveSessionIntegrations func(*session.Session) []interface{}
	hasFunctionIntegrations    func(keys []string) bool
}

type taskResult struct {
	SessionID string
	Profile   string
	Model     string
	Text      string
	Usage     *session.TokenUsage
	Error     string
}

func (tc *taskContext) executeNewTask(ctx context.Context, args StartTaskArgs) taskResult {
	keys := args.Integrations
	if len(keys) == 0 && args.Profile != "" {
		if p, err := tc.profiles.GetProfile(args.Profile); err == nil {
			keys = p.Integrations
		}
	}

	if tc.mcpPool != nil && tc.hasFunctionIntegrations != nil && tc.hasFunctionIntegrations(keys) {
		return tc.executeNewTaskWithToolProxy(ctx, args)
	}

	integrations, err := tc.profiles.ResolveProfileIntegrations(args.Profile, args.Integrations)
	if err != nil {
		return taskResult{Error: fmt.Sprintf("Integration error: %v", err)}
	}

	sampling := tc.profiles.ResolveSampling(args.Profile, args.Temperature)
	ctxLen := tc.profiles.ResolveContextLength(args.Profile, args.ContextLength)
	model := tc.profiles.ResolveModel(args.Profile)

	sess, err := tc.sessions.Create(args.Task, args.Profile, model, args.MaxTokens, args.Integrations, args.Project)
	if err != nil {
		return taskResult{Error: fmt.Sprintf("Session creation error: %v", err)}
	}
	if args.GroupID != "" {
		tc.sessions.SetGroup(sess.ID, args.GroupID, args.GroupStep)
		tc.registerSessionInGroup(sess.ID, args.GroupID)
	}

	systemPrompt := tc.profiles.AssembleSystemPrompt(args.Profile, args.SystemPrompt, args.Context, 0, sess.TokensMax)

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

	if tc.chatWriter != nil {
		tc.chatWriter.WriteUserMessage(sess.ID, fmt.Sprintf("%v", args.Task))
	}

	chatResp, err := tc.lm.ChatStream(ctx, chatReq, buildStreamCallbacks(tc.chatWriter, tc.artStore, tc.logger, sess.ID))
	if err != nil {
		if tc.chatWriter != nil {
			tc.chatWriter.WriteError(sess.ID, err.Error())
		}
		return taskResult{
			SessionID: sess.ID,
			Profile:   args.Profile,
			Model:     model,
			Error:     fmt.Sprintf("LM Studio error: %v", err),
		}
	}

	fullText := formatOutput(chatResp.Output)

	if tc.chatWriter != nil {
		tc.chatWriter.WriteComplete(sess.ID, fullText, &chatlog.ChatStats{
			InputTokens:  chatResp.Stats.InputTokens,
			OutputTokens: chatResp.Stats.TotalOutputTokens,
			TokensPerSec: chatResp.Stats.TokensPerSecond,
			ResponseID:   chatResp.ResponseID,
		})
	}

	_, usage, err := tc.sessions.AddTokens(sess.ID, chatResp.Stats.InputTokens, chatResp.Stats.TotalOutputTokens, chatResp.ResponseID)
	if err != nil {
		tc.logger.Printf("Token tracking error: %v", err)
	}

	return taskResult{
		SessionID: sess.ID,
		Profile:   args.Profile,
		Model:     chatResp.ModelInstanceID,
		Text:      fullText,
		Usage:     usage,
	}
}

func (tc *taskContext) executeNewTaskWithToolProxy(ctx context.Context, args StartTaskArgs) taskResult {
	sampling := tc.profiles.ResolveSampling(args.Profile, args.Temperature)
	ctxLen := tc.profiles.ResolveContextLength(args.Profile, args.ContextLength)
	model := tc.profiles.ResolveModel(args.Profile)

	sess, err := tc.sessions.Create(args.Task, args.Profile, model, args.MaxTokens, args.Integrations, args.Project)
	if err != nil {
		return taskResult{Error: fmt.Sprintf("Session creation error: %v", err)}
	}
	sess.UsesToolProxy = true
	tc.sessions.Save(sess)
	if args.GroupID != "" {
		tc.sessions.SetGroup(sess.ID, args.GroupID, args.GroupStep)
		tc.registerSessionInGroup(sess.ID, args.GroupID)
	}

	systemPrompt := tc.profiles.AssembleSystemPrompt(args.Profile, args.SystemPrompt, args.Context, 0, sess.TokensMax)
	tools := tc.mcpPool.AllToolDefs()

	req := &lmstudio.ResponsesRequest{
		Model:           model,
		Input:           args.Task,
		Instructions:    systemPrompt,
		Tools:           tools,
		Temperature:     sampling.Temperature,
		TopP:            sampling.TopP,
		MaxOutputTokens: sampling.MaxOutputTokens,
		ContextLength:   ctxLen,
		TopK:            sampling.TopK,
		MinP:            sampling.MinP,
		RepeatPenalty:   sampling.RepeatPenalty,
	}

	storeTrue := true
	req.Store = &storeTrue

	if tc.chatWriter != nil {
		tc.chatWriter.WriteUserMessage(sess.ID, fmt.Sprintf("%v", args.Task))
	}

	return tc.toolProxyLoop(ctx, sess, req, args.Profile)
}

func (tc *taskContext) executeContinueTaskWithToolProxy(ctx context.Context, sess *session.Session, message string) taskResult {
	tools := tc.mcpPool.AllToolDefs()

	req := &lmstudio.ResponsesRequest{
		Model:              sess.Model,
		Input:              message,
		Tools:              tools,
		PreviousResponseID: sess.LatestResponseID,
	}
	storeTrue := true
	req.Store = &storeTrue

	if tc.chatWriter != nil {
		tc.chatWriter.WriteUserMessage(sess.ID, message)
	}

	return tc.toolProxyLoop(ctx, sess, req, sess.Profile)
}

const maxToolRounds = 20

func (tc *taskContext) toolProxyLoop(ctx context.Context, sess *session.Session, req *lmstudio.ResponsesRequest, profileKey string) taskResult {
	model := req.Model

	for round := 0; round < maxToolRounds; round++ {
		var resp *lmstudio.ResponsesResponse
		var fullText strings.Builder
		var streamErr error

		resp, streamErr = tc.lm.ResponsesStream(ctx, req, lmstudio.ResponsesStreamCallbacks{
			OnTextDelta: func(delta string) {
				fullText.WriteString(delta)
				if tc.chatWriter != nil {
					tc.chatWriter.WriteDelta(sess.ID, delta)
				}
			},
			OnReasoningDelta: func(delta string) {
				if tc.chatWriter != nil {
					tc.chatWriter.WriteReasoningDelta(sess.ID, delta)
				}
			},
			OnError: func(message string) {
				if tc.chatWriter != nil {
					tc.chatWriter.WriteError(sess.ID, message)
				}
			},
		})

		if streamErr != nil {
			if tc.chatWriter != nil {
				tc.chatWriter.WriteError(sess.ID, streamErr.Error())
			}
			return taskResult{
				SessionID: sess.ID,
				Profile:   profileKey,
				Model:     model,
				Error:     fmt.Sprintf("LM Studio error: %v", streamErr),
			}
		}

		if resp.Usage != nil {
			tc.sessions.AddTokens(sess.ID, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.ID)
		} else if resp.ID != "" {
			tc.sessions.AddTokens(sess.ID, 0, 0, resp.ID)
		}

		// Always extract function calls from the completed response (streaming callbacks don't carry name/call_id)
		var functionCalls []lmstudio.ResponsesOutput
		for _, out := range resp.Output {
			if out.Type == "function_call" {
				functionCalls = append(functionCalls, out)
			}
		}

		if len(functionCalls) == 0 {
			text := fullText.String()
			if text == "" {
				for _, out := range resp.Output {
					if t := out.MessageText(); t != "" {
						text = t
					}
				}
			}

			if tc.chatWriter != nil {
				stats := &chatlog.ChatStats{ResponseID: resp.ID}
				if resp.Usage != nil {
					stats.InputTokens = resp.Usage.InputTokens
					stats.OutputTokens = resp.Usage.OutputTokens
				}
				tc.chatWriter.WriteComplete(sess.ID, text, stats)
			}

			sessFresh, _ := tc.sessions.Get(sess.ID)
			pct := float64(sessFresh.TokensUsed) / float64(sessFresh.TokensMax)
			usageResult := &session.TokenUsage{
				Used:       sessFresh.TokensUsed,
				Max:        sessFresh.TokensMax,
				Percentage: pct,
			}

			return taskResult{
				SessionID: sess.ID,
				Profile:   profileKey,
				Model:     model,
				Text:      text,
				Usage:     usageResult,
			}
		}

		// Execute each function call via MCP
		var followUpInputs []interface{}
		for _, fc := range functionCalls {
			if tc.chatWriter != nil {
				tc.chatWriter.WriteToolCallStart(sess.ID, fc.Name)
			}

			tc.logger.Printf("Executing tool %s (call_id=%s)", fc.Name, fc.CallID)

			var argsRaw json.RawMessage
			if fc.Arguments != "" {
				argsRaw = json.RawMessage(fc.Arguments)
			}

			result, callErr := tc.mcpPool.CallTool(fc.Name, argsRaw)

			success := callErr == nil
			output := result
			reason := ""
			if callErr != nil {
				output = callErr.Error()
				reason = callErr.Error()
			}

			if tc.chatWriter != nil {
				chatOutput := output
				if len(chatOutput) > 4096 {
					chatOutput = chatOutput[:4096] + "\n... (truncated in chatlog, full content in artifacts)"
				}
				tc.chatWriter.WriteToolCallResult(sess.ID, fc.Name, fc.Arguments, chatOutput, reason, success)
			}

			if tc.artStore != nil && success && output != "" {
				if err := tc.artStore.Store(sess.ID, fc.Name, argsRaw, output, nil); err != nil {
					tc.logger.Printf("Warning: failed to store artifact: %v", err)
				}
			}

			followUpInputs = append(followUpInputs, lmstudio.FunctionCallOutputInput{
				Type:   "function_call_output",
				CallID: fc.CallID,
				Output: output,
			})
		}

		// Send follow-up with tool results
		req = &lmstudio.ResponsesRequest{
			Model:              model,
			Input:              followUpInputs,
			Tools:              tc.mcpPool.AllToolDefs(),
			PreviousResponseID: resp.ID,
		}
		storeTrue := true
		req.Store = &storeTrue
	}

	return taskResult{
		SessionID: sess.ID,
		Profile:   profileKey,
		Model:     model,
		Error:     fmt.Sprintf("Tool calling loop exceeded %d rounds", maxToolRounds),
	}
}

func (tc *taskContext) executeContinueTask(ctx context.Context, sessionID, message string) taskResult {
	sess, err := tc.sessions.Get(sessionID)
	if err != nil {
		return taskResult{Error: fmt.Sprintf("Error: %v", err)}
	}
	if sess.Status != session.StatusActive {
		return taskResult{
			SessionID: sess.ID,
			Error:     fmt.Sprintf("Session %s is %s, not active.", sess.ID, sess.Status),
		}
	}

	if sess.UsesToolProxy && tc.mcpPool != nil {
		return tc.executeContinueTaskWithToolProxy(ctx, sess, message)
	}

	chatReq := &lmstudio.ChatRequest{
		Model: sess.Model,
		Input: message,
	}
	if ints := tc.resolveSessionIntegrations(sess); len(ints) > 0 {
		chatReq.Integrations = ints
	}
	if sess.LatestResponseID != "" {
		chatReq.PreviousResponseID = sess.LatestResponseID
	}

	if tc.chatWriter != nil {
		tc.chatWriter.WriteUserMessage(sess.ID, message)
	}

	chatResp, err := tc.lm.ChatStream(ctx, chatReq, buildStreamCallbacks(tc.chatWriter, tc.artStore, tc.logger, sess.ID))
	if err != nil {
		if tc.chatWriter != nil {
			tc.chatWriter.WriteError(sess.ID, err.Error())
		}
		return taskResult{
			SessionID: sess.ID,
			Profile:   sess.Profile,
			Model:     sess.Model,
			Error:     fmt.Sprintf("LM Studio error: %v", err),
		}
	}

	fullText := formatOutput(chatResp.Output)

	if tc.chatWriter != nil {
		tc.chatWriter.WriteComplete(sess.ID, fullText, &chatlog.ChatStats{
			InputTokens:  chatResp.Stats.InputTokens,
			OutputTokens: chatResp.Stats.TotalOutputTokens,
			TokensPerSec: chatResp.Stats.TokensPerSecond,
			ResponseID:   chatResp.ResponseID,
		})
	}

	_, usage, err := tc.sessions.AddTokens(sess.ID, chatResp.Stats.InputTokens, chatResp.Stats.TotalOutputTokens, chatResp.ResponseID)
	if err != nil {
		tc.logger.Printf("Token tracking error: %v", err)
	}

	return taskResult{
		SessionID: sess.ID,
		Profile:   sess.Profile,
		Model:     sess.Model,
		Text:      fullText,
		Usage:     usage,
	}
}

func grpID(g *taskgroup.Group) string {
	if g == nil {
		return ""
	}
	return g.ID
}

type chainStepResult struct {
	StepNum   int
	Profile   string
	SessionID string
	Text      string
}

func buildChainContext(results []chainStepResult, mode string, currentIdx int) string {
	if currentIdx == 0 || len(results) == 0 {
		return ""
	}

	var b strings.Builder

	if mode == "previous" {
		prev := results[len(results)-1]
		label := prev.Profile
		if label == "" {
			label = prev.SessionID
		}
		fmt.Fprintf(&b, "[Step %d - %s]:\n%s", prev.StepNum, label, prev.Text)
	} else {
		for _, r := range results {
			label := r.Profile
			if label == "" {
				label = r.SessionID
			}
			fmt.Fprintf(&b, "[Step %d - %s]:\n%s\n\n", r.StepNum, label, r.Text)
		}
	}

	return b.String()
}

func (tc *taskContext) registerSessionInGroup(sessionID, groupID string) {
	if tc.groups == nil {
		return
	}
	g, ok := tc.groups.Get(groupID)
	if !ok || g == nil {
		return
	}
	for _, sid := range g.SessionIDs {
		if sid == sessionID {
			return
		}
	}
	g.SessionIDs = append(g.SessionIDs, sessionID)
	tc.groups.Update(g)
}
