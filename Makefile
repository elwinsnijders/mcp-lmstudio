.PHONY: run build test clean install

run:
	go run cmd/mcp-lmstudio/main.go

build:
	go build -o mcp-lmstudio ./cmd/mcp-lmstudio/

install:
	go mod download
	go mod tidy

test: build
	go run cmd/test-client/main.go

clean:
	rm -f mcp-lmstudio
	rm -f /tmp/lmstudio_audit.log
	rm -rf sessions/ progress/
