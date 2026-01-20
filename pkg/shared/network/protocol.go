package network

import (
	"encoding/gob"
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
)

// RegisterGobTypes registers all types that will be sent over the wire.
func RegisterGobTypes() {
	gob.Register(LoginPacket{})
	gob.Register(LoginResponsePacket{})
	gob.Register(SignupPacket{})
	gob.Register(SignupResponsePacket{})
	gob.Register(UpdateKeybindingsPacket{})
	gob.Register(UpdateDebugSettingsPacket{})
	gob.Register(InputPacket{})
	gob.Register(StateUpdatePacket{})
	gob.Register(components.TransformComponent{})
	gob.Register(components.PhysicsComponent{})
	gob.Register(components.SpriteComponent{})
	gob.Register(components.InputComponent{})
	gob.Register(components.StatsComponent{})
	gob.Register(components.AttackComponent{})
	gob.Register(components.ProjectileComponent{})
	gob.Register(InventorySyncPacket{})
	gob.Register(InventoryActionPacket{})
	gob.Register(HotbarSyncPacket{})
	gob.Register(HotbarActionPacket{})
	gob.Register(HotbarSyncSlot{})
	gob.Register(EquipmentSyncPacket{})
	gob.Register(EquipmentActionPacket{})
	gob.Register(EquipmentActionPacket{})
	gob.Register(MapSyncPacket{})
	gob.Register(CastSpellPacket{})
	gob.Register(SpellbookSyncPacket{})
	gob.Register(UpdateUIStatePacket{})
}

type PacketType int

const (
	PacketLogin               PacketType = 1
	PacketLoginResponse       PacketType = 2
	PacketInput               PacketType = 3
	PacketStateUpdate         PacketType = 4
	PacketSignup              PacketType = 5
	PacketSignupResponse      PacketType = 6
	PacketUpdateKeybindings   PacketType = 7
	PacketInventorySync       PacketType = 8
	PacketInventoryAction     PacketType = 9
	PacketHotbarSync          PacketType = 10
	PacketHotbarAction        PacketType = 11
	PacketEquipmentSync       PacketType = 12
	PacketEquipmentAction     PacketType = 13
	PacketMapSync             PacketType = 14
	PacketUpdateDebugSettings PacketType = 15
	PacketCastSpell           PacketType = 16
	PacketSpellbookSync       PacketType = 17
	PacketUpdateUIState       PacketType = 18
)

// ... existing code ...

// UpdateDebugSettingsPacket (Client -> Server)
type UpdateDebugSettingsPacket struct {
	Settings map[string]bool
}

// UpdateUIStatePacket (Client -> Server)
type UpdateUIStatePacket struct {
	OpenMenus map[string]bool
}

// ... existing code ...

// HotbarSyncSlot
type HotbarSyncSlot struct {
	Type  string
	RefID string
}

// HotbarSyncPacket (Server -> Client)
type HotbarSyncPacket struct {
	Slots [10]HotbarSyncSlot
}

// HotbarActionPacket (Client -> Server)
type HotbarActionPacket struct {
	ActionType string // "Bind", "Swap", "Clear"
	SlotIndex  int    // Hotbar Slot (0-9)

	// For Bind:
	TargetType  string // "Item", "Spell"
	TargetRefID string // "potion_red"

	// For Swap:
	SlotIndexB int
}

// EquipmentSyncPacket (Server -> Client)
type EquipmentSyncPacket struct {
	Slots [9]struct {
		ItemID string
	}
}

// EquipmentActionPacket (Client -> Server)
type EquipmentActionPacket struct {
	Action string // "Equip", "Unequip"
	Slot   int    // Equipment Slot (0-8)
	// For Equip:
	InvSlot int // Inventory Slot (0-24)
}

type Packet struct {
	Type PacketType
	Data interface{}
}

// Client -> Server
type LoginPacket struct {
	Username string
	Password string
}

// Server -> Client
type LoginResponsePacket struct {
	Success        bool
	Error          string
	PlayerEntityID ecs.Entity
	PlayerX        float64
	PlayerY        float64
	MapWidth       int
	MapHeight      int
	MapTiles       []int
	MapObjects     []int
	UnlockedSpells []string
	Keybindings    map[string]int
	DebugSettings  map[string]bool
	OpenMenus      map[string]bool
}

// Client -> Server
type SignupPacket struct {
	Username string
	Password string
}

// Server -> Client
type SignupResponsePacket struct {
	Success bool
	Error   string
	Seed    int64
}

// Client -> Server
type UpdateKeybindingsPacket struct {
	Keybindings map[string]int
}

// Client -> Server
type InputPacket struct {
	Input components.InputComponent
}

// Server -> Client
type StateUpdatePacket struct {
	Entities []EntitySnapshot
}

type EntitySnapshot struct {
	ID        ecs.Entity
	Transform *components.TransformComponent
	Physics   *components.PhysicsComponent
	Sprite    *components.SpriteComponent
	Stats     *components.StatsComponent
}

// InventorySyncPacket (Server -> Client)
type InventorySyncPacket struct {
	Slots []struct {
		Index    int
		ItemID   string
		Quantity int
	}
	Capacity int
}

// InventoryActionPacket (Client -> Server)
type InventoryActionPacket struct {
	ActionType string // "Swap", "Drop", "Use"
	SlotA      int
	SlotB      int    // For swap
	ItemID     string // For drop/use (optional verification)
}

// MapSyncPacket (Server -> Client)
type MapSyncPacket struct {
	Level         int
	Width, Height int
	Tiles         []int // Flattened TileType array (Ground Layer)
	Objects       []int // Flattened ObjectType array (Objects Layer)
}

// CastSpellPacket (Client -> Server) - For Instant Casts
type CastSpellPacket struct {
	SpellID string // "heal"
}

// SpellbookSyncPacket (Server -> Client) - For Cooldowns and Unlocks
type SpellbookSyncPacket struct {
	UnlockedSpells []string
	Cooldowns      map[string]float64
}
