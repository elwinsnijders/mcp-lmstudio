.PHONY: run build test clean install

# Run the MCP server directly
run:
	go run cmd/mcp-lmstudio/main.go

# Build the MCP server binary
build:
	go build -o mcp-lmstudio cmd/mcp-lmstudio/main.go

# Install dependencies
install:
	go mod download
	go mod tidy

# Test the server (requires LM Studio running)
test: build
	./mcp-lmstudio

# Clean up binaries and logs
clean:
	rm -f mcp-lmstudio
	rm -f lmstudio_audit.log
