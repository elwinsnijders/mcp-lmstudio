.PHONY: run build test clean install ui ui-dev

UI = mcp-lmstudio-ui

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
	@if [ -d cmd/mcp-lmstudio-ui/build/bin/$(UI).app ]; then \
		rm -rf ./$(UI).app; \
		cp -r cmd/mcp-lmstudio-ui/build/bin/$(UI).app ./$(UI).app; \
	else \
		cp cmd/mcp-lmstudio-ui/build/bin/$(UI) ./$(UI); \
	fi

ui-dev:
	cd cmd/mcp-lmstudio-ui && wails dev

clean:
	rm -f mcp-lmstudio
	rm -f $(UI)
	rm -rf $(UI).app
	rm -f /tmp/lmstudio_audit.log
	rm -rf sessions/ progress/ chatlogs/
