package components

import (
	"henry/pkg/shared/ecs"
	"image/color"
)

// TransformComponent holds position and rotation
type TransformComponent struct {
	X, Y     float64
	Z        int     // Level (0=Ground, -1=Dungeon)
	Rotation float64 // in radians
}

// PhysicsComponent holds velocity and acceleration
type PhysicsComponent struct {
	VelX, VelY float64
	AccX, AccY float64
	Speed      float64 // Max speed or movement speed
}

type SpriteComponent struct {
	Color  color.RGBA
	Width  float64
	Height float64
}

// InputComponent holds the current input state for an entity
type InputComponent struct {
	Up, Down, Left, Right bool
	Attack                bool
	HotbarTriggers        [10]bool
	MouseX, MouseY        float64
	ActiveSpell           string // ID of the currently selected combat spell
}

// ... (other components)

// SpellbookComponent holds unlocked spells and cooldowns
type SpellbookComponent struct {
	UnlockedSpells []string
	Cooldowns      map[string]float64 // spellID -> lastCastTime (unix timestamp seconds)
}

// StatsComponent holds gameplay stats
type StatsComponent struct {
	MaxHealth     float64
	CurrentHealth float64
	Damage        float64
}

// InventorySlot represents a single slot in an inventory
type InventorySlot struct {
	ItemID   string
	Quantity int
}

// InventoryComponent holds the items for an entity
type InventoryComponent struct {
	Slots    []InventorySlot
	Capacity int
}

// HotbarSlot represents a reference in the hotbar
type HotbarSlot struct {
	Type  string // "Item", "Spell", etc.
	RefID string // e.g. "sword_starter"
}

// HotbarComponent holds hotbar configuration
type HotbarComponent struct {
	Slots [10]HotbarSlot
}

// Equipment Slots
const (
	SlotHead   = 0
	SlotNeck   = 1
	SlotBack   = 2
	SlotBody   = 3
	SlotLegs   = 4
	SlotWeapon = 5
	SlotShield = 6
	SlotFeet   = 7
	SlotHands  = 8
)

// EquipmentSlot represents a single worn item
type EquipmentSlot struct {
	ItemID string
}

// EquipmentComponent holds worn items
type EquipmentComponent struct {
	Slots [9]EquipmentSlot
}

// AIComponent holds state for NPC behavior
type AIComponent struct {
	Type           string     // "wander"
	State          string     // "idle", "move", "chase", "attack"
	StateTimer     float64    // Seconds remaining in current state
	MoveDirection  int        // 0:Up, 1:Down, 2:Left, 3:Right
	TargetID       ecs.Entity // Entity to attack
	IsAggressive   bool       // If true, auto-attacks
	Faction        int        // 0: Player, 1: Guards, 2: Monsters
	Path           [][]float64
	PathTimer      float64
	SpawnX, SpawnY float64
	LeashRange     float64
}

// RespawnComponent handles entity death and respawning
type RespawnComponent struct {
	SpawnX, SpawnY float64
	RespawnTimer   float64
	IsDead         bool
}

// UIStateComponent holds persistent UI visibility state
type UIStateComponent struct {
	OpenMenus map[string]bool
}
