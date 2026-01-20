# Makefile for Henry MMORPG

SERVER_BIN=server
CLIENT_WASM=static/client.wasm
CMD_SERVER=./cmd/server
CMD_CLIENT=./cmd/client

.PHONY: all build clean run kill restart

all: build

build: build-server build-client

build-server:
	@echo "Building Server..."
	go build -o $(SERVER_BIN) $(CMD_SERVER)

build-client:
	@echo "Building WASM Client..."
	GOOS=js GOARCH=wasm go build -o $(CLIENT_WASM) $(CMD_CLIENT)

clean:
	@echo "Cleaning..."
	rm -f $(SERVER_BIN) $(CLIENT_WASM)

kill:
	@echo "Killing existing server..."
	-killall $(SERVER_BIN) 2>/dev/null || true

run:
	@echo "Starting Server..."
	./$(SERVER_BIN)

restart: kill build run
