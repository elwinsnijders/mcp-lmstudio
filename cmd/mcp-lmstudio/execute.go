package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/infinitimeless/lmstudio-mcp/internal/artifacts"
	"github.com/infinitimeless/lmstudio-mcp/internal/chatlog"
	"github.com/infinitimeless/lmstudio-mcp/internal/lmstudio"
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
	resolveSessionIntegrations func(*session.Session) []interface{}
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
	integrations, err := tc.profiles.ResolveProfileIntegrations(args.Profile, args.Integrations)
	if err != nil {
		return taskResult{Error: fmt.Sprintf("Integration error: %v", err)}
	}

	sampling := tc.profiles.ResolveSampling(args.Profile, args.Temperature)
	ctxLen := tc.profiles.ResolveContextLength(args.Profile, args.ContextLength)
	model := tc.profiles.ResolveModel(args.Profile)

	sess, err := tc.sessions.Create(args.Task, args.Profile, model, args.MaxTokens, args.Integrations)
	if err != nil {
		return taskResult{Error: fmt.Sprintf("Session creation error: %v", err)}
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
