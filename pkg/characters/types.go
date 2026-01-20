package characters

import (
	"image/color"
)

// CharacterDefinition represents the static configuration for a character type.
// This acts as a Blueprint/Prefab for spawning entities.
type CharacterDefinition struct {
	ID          string // Unique ID e.g. "guard_melee"
	Name        string
	Description string

	// Visuals
	SpriteID     string // Asset Key e.g. "guard"
	SpriteWidth  float64
	SpriteHeight float64
	Color        color.RGBA

	// AI Configuration
	AIType       string // "wander", "guard", etc.
	Faction      int    // 0: Player, 1: Guards, 2: Monsters
	IsAggressive bool

	// Stats
	MaxHealth float64
	Speed     float64

	// Starting Equipment
	WeaponID string // e.g. "sword_starter"
}

var Registry = make(map[string]CharacterDefinition)

func Register(char CharacterDefinition) {
	if _, exists := Registry[char.ID]; exists {
		panic("Duplicate character ID: " + char.ID)
	}
	Registry[char.ID] = char
}

func Get(id string) (CharacterDefinition, bool) {
	c, ok := Registry[id]
	return c, ok
}
