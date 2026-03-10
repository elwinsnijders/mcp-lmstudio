# LMStudio-MCP (Go Implementation)

A Model Control Protocol (MCP) server written in Go that allows AI assistants to communicate with locally running LLM models via LM Studio.

## Overview

This is a Go port of the original Python-based LMStudio-MCP bridge. It provides the same functionality with improved performance and easier deployment as a single binary.

LMStudio-MCP creates a bridge between AI assistants (with MCP capabilities) and your locally running LM Studio instance. This allows AI tools to:

- Check the health of your LM Studio API
- List available models
- Get the currently loaded model
- Generate completions using your local models

This enables you to leverage your own locally running models through AI assistants, combining their capabilities with your private models.

## Prerequisites

- Go 1.24.0 or higher
- [LM Studio](https://lmstudio.ai/) installed and running locally with a model loaded
- MCP-compatible client (e.g., Claude Desktop, or any client supporting MCP)

## 🚀 Quick Installation

### Build from Source

```bash
cd mcp-lmstudio
make install  # Download dependencies
make build    # Build the binary
```

The binary `mcp-lmstudio` will be created in the current directory.

### Run Directly

```bash
make run
```

## MCP Configuration

### Using the Go Binary

Add this to your MCP client's configuration file.

**Example for Claude Desktop** (typically at `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "lmstudio-mcp": {
    "command": "/path/to/mcp-lmstudio/mcp-lmstudio"
  }
}
```

> **Note:** Configuration format may vary depending on your MCP client. Consult your client's documentation for specific instructions.

### Using Go Run (Development)

```json
{
  "lmstudio-mcp": {
    "command": "/bin/bash",
    "args": [
      "-c",
      "cd /path/to/mcp-lmstudio && go run cmd/mcp-lmstudio/main.go"
    ]
  }
}
```

## Usage

1. **Start LM Studio** and ensure it's running on port 1234 (the default)
2. **Load a model** in LM Studio
3. **Configure your MCP client** with one of the configurations above
4. **Connect to the MCP server** when prompted

## Available Tools

The bridge provides the following MCP tools:

### `health_check`
Check if LM Studio API is accessible.

**Returns:** A message indicating whether the LM Studio API is running.

### `list_models`
List all available models in LM Studio.

**Returns:** A formatted list of available models.

### `get_current_model`
Get the currently loaded model in LM Studio.

**Returns:** The name of the currently loaded model.

### `chat_completion`
Generate a completion from the current LM Studio model.

**Parameters:**
- `prompt` (required): The user's prompt to send to the model
- `system_prompt` (optional): System instructions for the model
- `temperature` (optional, default: 0.7): Controls randomness (0.0 to 1.0)
- `max_tokens` (optional, default: 1024): Maximum number of tokens to generate

**Returns:** The model's response to the prompt

## Configuration

### Environment Variables

- `LMSTUDIO_API_TOKEN`: Optional API token for authentication (if LM Studio requires it)

### API Base URL

By default, the server connects to `http://127.0.0.1:1234/v1`. To change this, modify the `LMStudioAPIBase` constant in `cmd/mcp-lmstudio/main.go`.

## Logging

All operations are logged to `lmstudio_audit.log` in the current directory. This includes:
- Tool executions
- API requests and responses
- Errors and debugging information

## Development

### Project Structure

```
mcp-lmstudio/
├── cmd/
│   └── mcp-lmstudio/
│       └── main.go          # Main server implementation
├── go.mod                   # Go module definition
├── go.sum                   # Go dependencies checksums
├── Makefile                 # Build automation
└── README_GO.md            # This file
```

### Building

```bash
make build
```

### Running

```bash
make run
```

### Cleaning

```bash
make clean  # Removes binary and log files
```

## Comparison with Python Version

### Advantages of Go Version

- **Single Binary**: No Python interpreter or virtual environment needed
- **Better Performance**: Faster startup and lower memory usage
- **Easy Deployment**: Just copy the binary, no dependency management
- **Cross-Platform**: Build for any platform Go supports
- **Static Typing**: Catch errors at compile time

### Compatibility

The Go version is fully compatible with the Python version and provides the same MCP tools and functionality.

## Troubleshooting

### API Connection Issues

If Claude reports 404 errors when trying to connect to LM Studio:
- Ensure LM Studio is running and has a model loaded
- Check that LM Studio's server is running on port 1234
- Verify your firewall isn't blocking the connection
- The server uses 127.0.0.1 to avoid IPv6 connection issues

### Build Issues

If you encounter build errors:
- Ensure you have Go 1.24.0 or higher: `go version`
- Run `make install` to download dependencies
- Check that you're in the correct directory

### Model Compatibility

If certain models don't work correctly:
- Some models might not fully support the OpenAI chat completions API format
- Try different parameter values (temperature, max_tokens) for problematic models
- Consider switching to a more compatible model if problems persist

## Contributing

Contributions are welcome! This Go implementation follows the same architecture as the original Python version.

## License

MIT

## Acknowledgements

This is a Go port of the original [LMStudio-MCP](https://github.com/infinitimeless/LMStudio-MCP) project by infinitimeless.

---

**🌟 If this project helps you, please consider giving it a star!**
