package characters

import "image/color"

func init() {
	// Melee Guard (Yellow)
	Register(CharacterDefinition{
		ID:           "guard_melee",
		Name:         "City Guard",
		Description:  "A generic city guard armed with a sword.",
		SpriteID:     "guard",
		SpriteWidth:  32,
		SpriteHeight: 32,
		Color:        color.RGBA{R: 255, G: 255, B: 0, A: 255}, // Yellow
		AIType:       "guard",
		Faction:      1,    // Guards
		IsAggressive: true, // Aggressive to monsters/enemies, but logic handles factions
		MaxHealth:    50,
		Speed:        1.0,
		WeaponID:     "sword_starter",
	})

	// Ranged Guard (Blue)
	Register(CharacterDefinition{
		ID:           "guard_ranged",
		Name:         "City Archer",
		Description:  "A sharpshooter guard armed with a bow.",
		SpriteID:     "guard",
		SpriteWidth:  32,
		SpriteHeight: 32,
		Color:        color.RGBA{R: 0, G: 0, B: 255, A: 255}, // Blue
		AIType:       "guard",
		Faction:      1, // Guards
		IsAggressive: true,
		MaxHealth:    40,
		Speed:        1.0,
		WeaponID:     "bow_starter",
	})
}
