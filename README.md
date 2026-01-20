# Henry MMORPG

A basic Open World MMORPG written in Go, featuring a custom ECS, authoritative server, and WebAssembly client.

## Tech Stack
- **Language**: Go (Golang) 1.22+
- **Graphics**: [Ebitengine v2](https://ebitengine.org/)
- **Networking**: TCP / WebSocket (using `encoding/gob`)
- **Architecture**: Entity Component System (ECS)

## Features
- **ECS Engine**: Custom-built Entity Component System in `pkg/core`.
- **WASM Client**: Runs in the browser, avoiding native dependency hell on Linux.
- **Authoritative Server**: Server handles physics, movement, and combat logic.
- **Combat**: Projectile-based combat with cooldowns and semi-auto firing.
- **Multiplayer**: Real-time position and state synchronization.

## How to Run

### Prerequisite
- Go installed.

### Quick Start
Use the included `Makefile` to build and run everything.

```bash
make restart
```

This will:
1.  Kill any running server instances.
2.  Compile the Server (`./server`).
3.  Compile the Client to WebAssembly (`static/client.wasm`).
4.  Start the Server.

Once running, open your browser to:
**http://localhost:8081**

### Controls
- **W.A.S.D**: Move Character
- **Mouse**: Aim
- **Left Click**: Attack (Semi-auto)
- **F1**: Toggle Debug Overlay

## Project Structure
- `cmd/server`: Game Server entry point.
- `cmd/client`: Game Client entry point (compiles to WASM).
- `pkg/core`: Shared game logic (ECS, Components, Physics).
- `pkg/network`: Networking protocol and wrappers.
- `static/`: HTML and WASM assets.
