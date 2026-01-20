package components

import "image/color"

type Spell struct {
	ID          string // Unique ID (e.g. "fireball")
	Name        string // Display Name
	Description string // Tooltip text
	Color       color.RGBA
	Icon        string  // Placeholder for icon ref if needed later
	CastTime    float64 // Seconds
	Cooldown    float64 // Seconds
	Type        string  // "combat", "instant"
}

var SpellRegistry = map[string]Spell{
	"fireball": {
		ID:          "fireball",
		Name:        "Fireball",
		Description: "Launches a fiery ball dealing damage.",
		Color:       color.RGBA{255, 100, 50, 255}, // Orange/Red
		Icon:        "fireball",
		Cooldown:    2.0,
		Type:        "combat",
	},
	"heal": {
		ID:          "heal",
		Name:        "Heal",
		Description: "Restores a small amount of health.",
		Color:       color.RGBA{100, 255, 100, 255}, // Green
		Cooldown:    5.0,
		Type:        "instant",
	},
	"blink": {
		ID:          "blink",
		Name:        "Blink",
		Description: "Teleports you short distance forward.",
		Color:       color.RGBA{100, 100, 255, 255}, // Blue
		Cooldown:    8.0,
		Type:        "instant",
	},
	"shield": {
		ID:          "shield",
		Name:        "Mana Shield",
		Description: "Absorbs damage using mana.",
		Color:       color.RGBA{200, 200, 255, 255}, // Light Blue
		Cooldown:    15.0,
		Type:        "instant",
	},
	"void": {
		ID:          "void",
		Name:        "Void Walk",
		Description: "Become invisible for a short time.",
		Color:       color.RGBA{100, 0, 100, 255}, // Purple
		Cooldown:    20.0,
		Type:        "instant",
	},
}

// Ordered list for UI display consistency
var SpellList = []string{
	"fireball",
	"heal",
	"blink",
	"shield",
	"void",
}
