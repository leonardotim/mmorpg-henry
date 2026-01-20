# AGENT.md - Context for AI Developers

This file contains high-level context, architectural decisions, and "gotchas" to assist future AI agents in working on this codebase.

## Architecture Overview

### 1. ECS (Entity Component System)
- **Location**: `pkg/core`
- **Implementation**: Custom, simplified ECS.
- **Entities**: `uint64` IDs.
- **Components**: Stored in `World.components` as `map[reflect.Type]map[Entity]Component`.
- **Querying**: Use `core.Query[ComponentType](world)` to get a list of Entities.
- **Caveat**: `GetComponent` returns a *pointer* to a copy (due to interface casting quirks) or the struct itself. Be careful when modifying components; always write back using `AddComponent`.

### 2. Networking
- **Location**: `pkg/network`
- **Protocol**: `encoding/gob` over TCP or WebSocket.
- **Dual Stack**: 
    - Native TCP Client (Desktop) uses `net.Dial`.
    - WASM Client (Browser) uses `websocket.Dial` via `pkg/network/dial_wasm.go`.
    - Server listens on both (:8080 TCP, :8081 WS).
- **State Sync**: Server broadcasts `StateUpdatePacket` (snapshot of all visible entities) at 30Hz.

### 3. Client (WebAssembly)
- **Engine**: Ebitengine (v2).
- **Build Tag**: `GOOS=js GOARCH=wasm`.
- **Input**: Mouse and Keyboard logic is in `cmd/client/main.go`.
- **Debug**: F1 toggles debug overlay.

### 4. Server
- **Loop**: Fixed timestep (approx 30 TPS).
- **Physics**: Simple AABB/Point logic. No spatial partition (O(N^2) checks possible if not careful).
- **Combat**: 
    - `HandleAttack` logic in `cmd/server/main.go`.
    - Enforces Cooldowns and Semi-Auto fire (Edge detection on Input).

## Critical Implementation Details

### Deployment / Running
- Always use `make restart` to rebuild WASM and restart Server.
- Client **MUST** be served via HTTP (cannot run WASM from file://).
- Server serves `static/` directory on port 8081.

### Known Technical Debt / TODOs
1.  **ECS Performance**: `core.Query` iterates maps. Not optimized for 1000+ entities.
2.  **Broadcasting**: We send the *entire* world state to *every* player every tick. Bandwidth heavy. Needs Interest Management (AOI).
3.  **Input Prediction**: Client is currently dumb terminals (send input -> wait for state). No client-side prediction means latency is felt.
4.  **Collision**: Basic primitive checks in `pkg/core/combat.go`.
5.  **Hardcoded Gameplay**: Much of the logic (Movement, Attack handling) is currently hardcoded in `cmd/server/main.go` methods instead of separated Systems.

## AI Agent Guidelines
1. **No Manual Testing**: Do NOT attempt to perform manual verification or testing using browser subagents or automated browser tools. The USER prefers to handle verification themselves. Provide the build and setup, then ask the USER to verify.
2. **Post-Task Restart**: Always run `make restart` after completing a task to ensure the server and WASM client are up-to-date and running for the user.

## Common Patterns
- **Adding a Component**: 
    1. Define struct in `pkg/core/components.go`.
    2. Register in `pkg/network/protocol.go` (Gob).
    3. Add to Entity in Server.
