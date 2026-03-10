# Quick Start Guide - Go Implementation

Get up and running with the Go implementation of LMStudio-MCP in 5 minutes.

## Prerequisites Check

Before starting, verify you have:

```bash
# Check Go version (need 1.24.0+)
go version

# Check LM Studio is running
curl http://localhost:1234/v1/models
```

If LM Studio returns a response, you're ready!

## Step 1: Build

```bash
# Navigate to the project
cd /path/to/mcp-lmstudio

# Install dependencies and build
make install
make build
```

You should see: `mcp-lmstudio` binary created (~11MB).

## Step 2: Test the Binary

```bash
# Test that the binary runs (it will wait for MCP protocol input)
# Press Ctrl+C to exit after a second
./mcp-lmstudio
```

## Step 3: Configure Your MCP Client

1. **Find your MCP client's config file:**
   
   **Example for Claude Desktop:**
   - **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`
   - **Linux:** `~/.config/Claude/claude_desktop_config.json`

2. **Add the MCP server configuration:**

```json
{
  "mcpServers": {
    "lmstudio": {
      "command": "/path/to/mcp-lmstudio/mcp-lmstudio"
    }
  }
}
```

Replace `/path/to/mcp-lmstudio/mcp-lmstudio` with the actual absolute path to your binary.

> **Note:** Configuration format may vary depending on your MCP client. Consult your client's documentation for specific instructions.

3. **Restart your MCP client**

## Step 4: Verify Connection

1. Open your MCP client
2. Look for the MCP server indicator
3. You should see "lmstudio" in the list of available servers
4. Click to connect

## Step 5: Test the Tools

Try these commands:

```
Can you check if LM Studio is running?
```

The AI should use the `health_check` tool and report that LM Studio is accessible.

```
What models are available in LM Studio?
```

The AI should use the `list_models` tool.

```
Use LM Studio to answer: What is the capital of France?
```

The AI should use the `chat_completion` tool to ask your local model.

## Troubleshooting

### Binary Won't Run

```bash
# Check if binary is executable
ls -l mcp-lmstudio

# Make it executable if needed
chmod +x mcp-lmstudio
```

### MCP Client Can't Find the Binary

1. Use **absolute path** in config (not relative)
2. Verify path exists: `ls /full/path/to/mcp-lmstudio`
3. Check permissions: `./mcp-lmstudio` should work

### Connection Errors

```bash
# Verify LM Studio is running
curl http://localhost:1234/v1/models

# Check logs
tail -f lmstudio_audit.log
```

### Build Errors

```bash
# Clean and rebuild
make clean
rm -rf go.sum
make install
make build
```

## Advanced Configuration

### Custom API URL

Edit `cmd/mcp-lmstudio/main.go`:

```go
const (
    LMStudioAPIBase = "http://your-custom-host:port/v1"
)
```

Then rebuild:

```bash
make build
```

### API Authentication

Set environment variable in your shell:

```bash
export LMSTUDIO_API_TOKEN="your-token-here"
```

Or add to MCP client config:

```json
{
  "mcpServers": {
    "lmstudio": {
      "command": "/path/to/mcp-lmstudio/mcp-lmstudio",
      "env": {
        "LMSTUDIO_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Cross-Platform Builds

Build for different platforms:

```bash
# For Linux (AMD64)
GOOS=linux GOARCH=amd64 go build -o mcp-lmstudio-linux cmd/mcp-lmstudio/main.go

# For Windows
GOOS=windows GOARCH=amd64 go build -o mcp-lmstudio.exe cmd/mcp-lmstudio/main.go

# For macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o mcp-lmstudio-intel cmd/mcp-lmstudio/main.go

# For macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o mcp-lmstudio-arm cmd/mcp-lmstudio/main.go
```

## Next Steps

- Read [README_GO.md](README_GO.md) for complete documentation
- Check [COMPARISON.md](COMPARISON.md) to understand Go vs Python differences
- See [MCP_CONFIGURATION.md](MCP_CONFIGURATION.md) for advanced MCP setup
- Review [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues

## Success!

If your AI assistant can successfully use any of the tools, you're all set! The Go implementation is:

- ✅ Running as a single binary
- ✅ Connected to your LM Studio instance
- ✅ Available through MCP
- ✅ Logging to `lmstudio_audit.log`

Enjoy using your local models!
