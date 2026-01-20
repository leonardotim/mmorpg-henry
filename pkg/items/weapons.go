package items

import (
	"henry/pkg/shared/components"
)

func init() {
	// Melee Weapons
	Register(ItemDefinition{
		ID:          "sword_starter",
		Name:        "Rusty Sword",
		Type:        ItemTypeWeapon,
		Description: "A basic sword using close combat slash attacks.",
		WeaponStats: &components.AttackComponent{
			Damage:   20,
			Range:    60,
			Cooldown: 0.8,
			Type:     components.AttackTypeMelee,
		},
		EquipmentSlot: components.SlotWeapon,
	})

	// Ranged Weapons
	Register(ItemDefinition{
		ID:          "bow_starter",
		Name:        "Old Bow",
		Type:        ItemTypeWeapon,
		Description: "A worn bow for ranged attacks.",
		WeaponStats: &components.AttackComponent{
			Damage:   10,
			Range:    400,
			Cooldown: 0.5,
			Type:     components.AttackTypeRanged,
		},
		EquipmentSlot: components.SlotWeapon,
	})
}
