# MCP LM Studio Orchestrator

An MCP server that lets AI orchestrators (Claude Opus in Cursor) delegate execution tasks to worker AIs running on LM Studio. Manages sessions, tracks token budgets, maintains stateful conversations, and persists progress between worker shifts.

## Architecture

**Claude Opus** is the architect -- it decides what to build. **Worker AI** (LM Studio) is the builder -- it does the actual work. This MCP server is the foreman -- it manages sessions, tracks budgets, and handles handoffs.

```
Claude Opus (Cursor)
    |  MCP Protocol (stdio)
    v
MCP Server (this binary)
    |  - Session Manager (tracks sessions, tokens, response chains)
    |  - Profile System (agent roles, system prompts, integrations)
    |  - Progress Manager (markdown progress files)
    |  HTTP REST
    v
LM Studio (local AI)
    |  - Stateful Chats (response_id continuity)
    |  - MCP Integrations (filesystem, web, etc.)
```

## Quick Start

1. **Install LM Studio** and load a model
2. **Build the server:**
   ```bash
   make build
   ```
3. **Configure** `config.json` with your profiles and integrations
4. **Add to Cursor's MCP config** (see Configuration below)

## MCP Tools

### Discovery
| Tool | Description |
|------|-------------|
| `health_check` | Check if LM Studio API is accessible |
| `list_models` | List available models in LM Studio |
| `list_profiles` | List agent profiles (coder, reviewer, etc.) |
| `list_integrations` | List available integrations (filesystem, etc.) |

### Task Lifecycle
| Tool | Description |
|------|-------------|
| `start_task` | Start a new worker session. Pass `task` + `profile` key. |
| `continue_task` | Continue a session. Worker remembers the conversation. |
| `end_session` | End a session, optionally saving progress first. |

### Progress
| Tool | Description |
|------|-------------|
| `save_progress` | Ask worker to summarize, then save to markdown file. |
| `load_progress` | Load a progress file to use as context for a new session. |

### Session Management
| Tool | Description |
|------|-------------|
| `get_session_status` | Get token usage and metadata for a session. |
| `list_sessions` | List all sessions with status and token counts. |

## Typical Workflow

```
1. Claude calls start_task(task: "Refactor auth to JWT", profile: "coder")
   -> Returns: session_id, worker response, token usage

2. Claude calls continue_task(session_id, "Use RS256 algorithm")
   -> Returns: worker response, updated token count

3. When approaching token limit, Claude calls save_progress(session_id)
   -> Returns: progress file path

4. Claude calls start_task(task: "Continue refactoring", profile: "coder", context: <progress file>)
   -> New session picks up where previous left off
```

## Configuration

### config.json

The `config.json` file defines agent profiles and integrations. Claude only needs to pass short keys like `profile: "coder"` -- all system prompts and settings resolve server-side.

```json
{
  "shared_system_prompt": "Follow clean code principles. Be concise in explanations. When modifying files, always verify the current state before making changes.",

  "profiles": {
    "coder": {
      "label": "Senior Developer",
      "description": "Implements features, writes clean production code, refactors existing code",
      "system_prompt": "You are a senior software developer. Implement the given task with clean, production-quality code. Use the tools available to read existing code and write your changes directly. Focus on code, not narration.",
      "model": "qwen3.5-27b",
      "temperature": 0.6,
      "context_length": 175000,
      "top_p": 0.95,
      "top_k": 20,
      "min_p": 0,
      "repeat_penalty": 1.0,
      "integrations": ["sj-sandbox"]
    },
    "reviewer": {
      "label": "Code Reviewer",
      "description": "Reviews code for bugs, security issues, performance, and best practices",
      "system_prompt": "You are an expert code reviewer. Analyze the given code or changeset for bugs, security vulnerabilities, performance issues, and deviations from best practices. Structure your review as: Critical Issues, Warnings, Suggestions.",
      "model": "qwen3.5-27b@q4_k_s",
      "temperature": 0.6,
      "context_length": 175000,
      "max_output_tokens": 175000,
      "reasoning": "off",
      "integrations": ["sj-sandbox"]
    },
    "tester": {
      "label": "Test Engineer",
      "description": "Writes unit tests, integration tests, identifies edge cases and missing coverage",
      "system_prompt": "You are a QA engineer. Write thorough tests for the given code. Cover happy paths, edge cases, error conditions, and boundary values. Use the tools available to read source code and write test files directly.",
      "model": "qwen3.5-27b@q4_k_s",
      "temperature": 0.6,
      "context_length": 175000,
      "max_output_tokens": 175000,
      "reasoning": "off",
      "integrations": ["sj-sandbox"]
    },
    "researcher": {
      "label": "Codebase Researcher",
      "description": "Explores codebases, documents architecture, maps dependencies, finds patterns",
      "system_prompt": "You are a codebase analyst. Explore the given codebase and answer questions about its structure, patterns, and architecture. Be thorough in exploration but concise in reports.",
      "model": "qwen3.5-27b@q4_k_s",
      "temperature": 0.6,
      "context_length": 175000,
      "max_output_tokens": 175000,
      "reasoning": "off",
      "integrations": ["sj-sandbox"]
    },
    "debugger": {
      "label": "Debugger",
      "description": "Traces errors to root causes, analyzes stack traces, proposes and applies fixes",
      "system_prompt": "You are a debugging specialist. Trace the reported error to its root cause. Read source code, examine related files, and understand the call chain. Report: Root Cause, Evidence, Fix, Prevention.",
      "model": "qwen3.5-27b@q4_k_s",
      "temperature": 0.6,
      "context_length": 175000,
      "max_output_tokens": 175000,
      "reasoning": "off",
      "integrations": ["sj-sandbox"]
    }
  },

  "integrations": {
    "sj-sandbox": {
      "label": "JS code sandbox",
      "description": "Try and test js code",
      "type": "plugin",
      "id": "mcp/js-sandbox"
    },
    "filesystem": {
      "label": "Filesystem Access",
      "description": "Read/write project files and list directories",
      "type": "plugin",
      "id": "mcp/filesystem"
    },
    "playwright": {
      "label": "Browser Automation",
      "description": "Navigate web pages, interact with elements, take screenshots",
      "type": "plugin",
      "id": "mcp/playwright"
    }
  }
}
```

**System prompt assembly** (5 layers, concatenated in order):
1. Efficiency preamble (hardcoded, always on)
2. Shared system prompt (from config, applies to all profiles)
3. Profile prompt (role-specific, from config)
4. Override prompt (optional, from Claude per-call)
5. Context (optional, e.g. progress from a previous session)

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LMSTUDIO_API_BASE` | `http://127.0.0.1:1234` | LM Studio URL |
| `LMSTUDIO_API_TOKEN` | (none) | Bearer token for auth |
| `LMSTUDIO_MODEL` | `default` | Default model name |
| `LMSTUDIO_CONTEXT_LENGTH` | `8192` | Default context window |
| `LMSTUDIO_REQUEST_TIMEOUT` | `10` | HTTP timeout in minutes |
| `MAX_SESSION_TOKENS` | `175000` | Token budget per session |
| `TOKEN_WARNING_THRESHOLD` | `0.80` | Warning at this % |
| `TOKEN_CRITICAL_THRESHOLD` | `0.95` | Critical warning at this % |
| `CONFIG_FILE` | `config.json` | Path to config file |
| `SESSIONS_DIR` | `sessions/` | Session state directory |
| `PROGRESS_DIR` | `progress/` | Progress files directory |
| `LOG_FILE` | `/tmp/lmstudio_audit.log` | Audit log path |

### Cursor MCP Configuration

Add to your Cursor MCP settings:

```json
{
  "mcpServers": {
    "lmstudio-bridge": {
      "command": "/path/to/mcp-lmstudio",
      "env": {
        "LMSTUDIO_MODEL": "your-model-key",
        "CONFIG_FILE": "/path/to/config.json"
      }
    }
  }
}
```

## Token Management

Every response includes token usage. When approaching the configured limit:

- **80%** -- `WARNING: Token usage at 80%. Consider saving progress.`
- **95%** -- `CRITICAL: Token usage at 95%. Save progress immediately.`

The `save_progress` tool asks the worker for a structured summary, writes it to a markdown file, and returns the path. Use `load_progress` to feed it as context into a new session via `start_task`.

## Built-in Agent Profiles

| Profile | Role | Model | Temp | Integrations |
|---------|------|-------|------|--------------|
| `coder` | Senior Developer | `qwen3.5-27b` | 0.6 | sj-sandbox |
| `reviewer` | Code Reviewer | `qwen3.5-27b@q4_k_s` | 0.6 | sj-sandbox |
| `tester` | Test Engineer | `qwen3.5-27b@q4_k_s` | 0.6 | sj-sandbox |
| `researcher` | Codebase Researcher | `qwen3.5-27b@q4_k_s` | 0.6 | sj-sandbox |
| `debugger` | Debugger | `qwen3.5-27b@q4_k_s` | 0.6 | sj-sandbox |

All profiles are customizable in `config.json`. Each profile supports per-model settings including `context_length`, `top_p`, `top_k`, `min_p`, `repeat_penalty`, `max_output_tokens`, and `reasoning`.

## Development

```bash
make install   # Download dependencies
make build     # Build binary
make test      # Run test client (requires LM Studio running)
make clean     # Remove binary and data directories
```
