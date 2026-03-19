GOEXE   := $(shell go env GOEXE)
SERVER  := mcp-lmstudio$(GOEXE)
UI      := mcp-lmstudio-ui$(GOEXE)

.PHONY: all build server ui ui-dev install test clean

all: build

build: server ui
	@echo "Built $(SERVER) and $(UI) in project root."

server:
	go build -o $(SERVER) ./cmd/mcp-lmstudio/

ui:
	cd cmd/mcp-lmstudio-ui && wails build
	@if [ -d cmd/mcp-lmstudio-ui/build/bin/$(UI).app ]; then \
		rm -rf ./$(UI).app; \
		cp -r cmd/mcp-lmstudio-ui/build/bin/$(UI).app ./$(UI).app; \
	else \
		cp cmd/mcp-lmstudio-ui/build/bin/$(UI) ./$(UI); \
	fi

ui-dev:
	cd cmd/mcp-lmstudio-ui && wails dev

install: build
	@if [ ! -f config.json ]; then \
		cp config.json.example config.json; \
		echo "Created config.json from example."; \
	else \
		echo "config.json already exists, skipping."; \
	fi
	@echo ""
	@echo "Done! Next steps:"
	@echo "  1. Edit config.json (or run ./$(UI) to configure via the UI)"
	@echo "  2. Add mcp-lmstudio to your AI client's MCP config"
	@echo "  3. See README.md for setup examples"

test: server
	go run cmd/test-client/main.go

clean:
	rm -f $(SERVER) $(UI)
	rm -rf $(UI).app
	rm -f /tmp/lmstudio_audit.log
	rm -rf sessions/ progress/ chatlogs/
