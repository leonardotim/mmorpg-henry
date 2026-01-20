package items

import "henry/pkg/shared/components"

type ItemType int

const (
	ItemTypeWeapon ItemType = iota
	ItemTypeConsumable
	ItemTypeMisc
)

// ItemDefinition represents the static data for an item.
type ItemDefinition struct {
	ID          string // Unique string ID e.g. "sword_rusty"
	Name        string
	Type        ItemType
	Description string

	// Component Data (Optional, depending on Type)
	WeaponStats *components.AttackComponent

	// Equipment Data
	EquipmentSlot int // -1 if not equippable
}

var Registry = make(map[string]ItemDefinition)

func Register(item ItemDefinition) {
	if _, exists := Registry[item.ID]; exists {
		panic("Duplicate item ID: " + item.ID)
	}
	Registry[item.ID] = item
}

func Get(id string) (ItemDefinition, bool) {
	item, ok := Registry[id]
	return item, ok
}
