.PHONY: run build test clean install ui ui-dev

run:
	go run cmd/mcp-lmstudio/main.go

build:
	go build -o mcp-lmstudio ./cmd/mcp-lmstudio/

install:
	go mod download
	go mod tidy

test: build
	go run cmd/test-client/main.go

ui:
	cd cmd/mcp-lmstudio-ui && wails build

ui-dev:
	cd cmd/mcp-lmstudio-ui && wails dev

clean:
	rm -f mcp-lmstudio
	rm -f /tmp/lmstudio_audit.log
	rm -rf sessions/ progress/ chatlogs/
