package components

import (
	"henry/pkg/shared/ecs"
	"math"
)

type AttackType int

const (
	AttackTypeMelee AttackType = iota
	AttackTypeRanged
)

type AttackComponent struct {
	Damage         float64
	Range          float64
	Cooldown       float64 // Seconds
	LastAttackTime float64 // Seconds since game start or unix timestamp
	Type           AttackType
}

type ProjectileComponent struct {
	OwnerID  ecs.Entity
	Damage   float64
	Lifetime float64
}

// Simple Collision Check (Circle/Point)
func CheckCollision(x1, y1, r1, x2, y2, r2 float64) bool {
	dx := x2 - x1
	dy := y2 - y1
	distSq := dx*dx + dy*dy
	radiusSum := r1 + r2
	return distSq <= radiusSum*radiusSum
}

// Calculate direction vector
func Direction(x1, y1, x2, y2 float64) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	msg := math.Sqrt(dx*dx + dy*dy)
	if msg == 0 {
		return 0, 0
	}
	return dx / msg, dy / msg
}
