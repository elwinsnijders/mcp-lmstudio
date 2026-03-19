package profile

import (
	"fmt"
	"strings"

	"github.com/infinitimeless/lmstudio-mcp/internal/config"
	"github.com/infinitimeless/lmstudio-mcp/internal/lmstudio"
)

const preambleTemplate = `You are an AI assistant working on a delegated task. Token budget: %d/%d tokens.
Be concise. Focus on code and results, not narration. Document errors with messages and fixes.
Track which files you read and modify. Use your tools to access files directly.
When you read files, do not reproduce their full contents in your response. The orchestrator can retrieve file contents directly from captured tool results. Focus on analysis, actions, and relevant snippets.`

type Manager struct {
	cfg *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) GetProfile(key string) (*config.ProfileConfig, error) {
	p, ok := m.cfg.App.Profiles[key]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", key)
	}
	return &p, nil
}

func (m *Manager) ListProfiles() map[string]config.ProfileConfig {
	return m.cfg.App.Profiles
}

func (m *Manager) ListIntegrations() map[string]config.IntegrationConfig {
	return m.cfg.App.Integrations
}

func (m *Manager) AssembleSystemPrompt(profileKey, overridePrompt, ctx string, tokensUsed, tokensMax int) string {
	var parts []string

	parts = append(parts, fmt.Sprintf(preambleTemplate, tokensUsed, tokensMax))

	if m.cfg.App.SharedSystemPrompt != "" {
		parts = append(parts, m.cfg.App.SharedSystemPrompt)
	}

	if overridePrompt != "" {
		parts = append(parts, overridePrompt)
	} else if profileKey != "" {
		if p, ok := m.cfg.App.Profiles[profileKey]; ok && p.SystemPrompt != "" {
			parts = append(parts, p.SystemPrompt)
		}
	}

	if ctx != "" {
		parts = append(parts, "--- CONTEXT FROM PREVIOUS SESSION ---\n"+ctx)
	}

	return strings.Join(parts, "\n\n")
}

func (m *Manager) ResolveIntegrations(keys []string) ([]interface{}, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	var result []interface{}
	for _, key := range keys {
		ic, ok := m.cfg.App.Integrations[key]
		if !ok {
			return nil, fmt.Errorf("integration %q not found in config", key)
		}

		switch ic.Type {
		case "plugin":
			result = append(result, lmstudio.PluginIntegration{
				Type:         "plugin",
				ID:           ic.ID,
				AllowedTools: ic.AllowedTools,
			})
		case "ephemeral_mcp":
			label := ic.ServerLabel
			if label == "" {
				label = key
			}
			result = append(result, lmstudio.EphemeralMCPIntegration{
				Type:         "ephemeral_mcp",
				ServerLabel:  label,
				ServerURL:    ic.ServerURL,
				AllowedTools: ic.AllowedTools,
				Headers:      ic.Headers,
			})
		default:
			return nil, fmt.Errorf("integration %q has unknown type %q", key, ic.Type)
		}
	}

	return result, nil
}

// ResolveProfileIntegrations resolves integration keys, falling back to profile defaults.
func (m *Manager) ResolveProfileIntegrations(profileKey string, explicitKeys []string) ([]interface{}, error) {
	keys := explicitKeys
	if len(keys) == 0 && profileKey != "" {
		if p, ok := m.cfg.App.Profiles[profileKey]; ok {
			keys = p.Integrations
		}
	}
	return m.ResolveIntegrations(keys)
}

func (m *Manager) ResolveModel(profileKey string) string {
	if profileKey != "" {
		if p, ok := m.cfg.App.Profiles[profileKey]; ok && p.Model != "" {
			return p.Model
		}
	}
	return m.cfg.Model
}

func (m *Manager) ResolveContextLength(profileKey string, explicit int) int {
	if explicit > 0 {
		return explicit
	}
	if profileKey != "" {
		if p, ok := m.cfg.App.Profiles[profileKey]; ok && p.ContextLength > 0 {
			return p.ContextLength
		}
	}
	return m.cfg.ContextLength
}

type SamplingParams struct {
	Temperature     *float64
	TopP            *float64
	TopK            *int
	MinP            *float64
	RepeatPenalty   *float64
	MaxOutputTokens *int
	Reasoning       string
}

func (m *Manager) ResolveSampling(profileKey string, tempOverride float64) SamplingParams {
	var sp SamplingParams

	if profileKey != "" {
		if p, ok := m.cfg.App.Profiles[profileKey]; ok {
			sp = samplingFromProfile(p)
		}
	}

	if tempOverride > 0 {
		sp.Temperature = &tempOverride
	}

	return sp
}

func samplingFromProfile(p config.ProfileConfig) SamplingParams {
	var sp SamplingParams
	if p.Temperature > 0 {
		sp.Temperature = &p.Temperature
	}
	if p.TopP > 0 {
		sp.TopP = &p.TopP
	}
	if p.TopK > 0 {
		sp.TopK = &p.TopK
	}
	if p.MinP > 0 {
		sp.MinP = &p.MinP
	}
	if p.RepeatPenalty > 0 {
		sp.RepeatPenalty = &p.RepeatPenalty
	}
	if p.MaxOutputTokens > 0 {
		sp.MaxOutputTokens = &p.MaxOutputTokens
	}
	sp.Reasoning = p.Reasoning
	return sp
}
