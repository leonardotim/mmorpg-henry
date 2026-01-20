package systems

import (
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
	"henry/pkg/shared/world"
	"math"
)

type MovementSystem struct {
	World *ecs.World
	Maps  map[int]*world.Map
}

func NewMovementSystem(world *ecs.World, atlas map[int]*world.Map) *MovementSystem {
	return &MovementSystem{
		World: world,
		Maps:  atlas,
	}
}

func (s *MovementSystem) Update(dt float64) {
	// Query all entities with Input, Transform, and Physics components
	entities := ecs.Query[components.InputComponent](s.World)
	for _, id := range entities {
		s.UpdateEntityMovement(id)
	}
}

func (s *MovementSystem) UpdateEntityMovement(id ecs.Entity) {
	input, _ := ecs.GetComponent[components.InputComponent](s.World, id)
	transform, _ := ecs.GetComponent[components.TransformComponent](s.World, id)
	phys, _ := ecs.GetComponent[components.PhysicsComponent](s.World, id)

	if input == nil || transform == nil || phys == nil {
		return
	}

	dx, dy := 0.0, 0.0
	if input.Up {
		dy = -1
	}
	if input.Down {
		dy = 1
	}
	if input.Left {
		dx = -1
	}
	if input.Right {
		dx = 1
	}

	// Normalize diagonal movement
	if dx != 0 && dy != 0 {
		dx *= 0.7071
		dy *= 0.7071
	}

	moveX := dx * phys.Speed
	moveY := dy * phys.Speed

	// Collision box (centered in 32x32 sprite)
	boxSize := 14.0 // Reduced for better feel (was 20.0)
	offset := (32.0 - boxSize) / 2.0

	z := transform.Z

	// Try move X
	if !s.collidesAt(z, transform.X+moveX+offset, transform.Y+offset, boxSize, boxSize) &&
		!s.collidesWithEntities(id, z, transform.X+moveX+offset, transform.Y+offset, boxSize, boxSize) {
		transform.X += moveX
	}

	// Try move Y
	if !s.collidesAt(z, transform.X+offset, transform.Y+moveY+offset, boxSize, boxSize) &&
		!s.collidesWithEntities(id, z, transform.X+offset, transform.Y+moveY+offset, boxSize, boxSize) {
		transform.Y += moveY
	}

	// Update Rotation
	transform.Rotation = math.Atan2(input.MouseY-transform.Y, input.MouseX-transform.X)

	s.World.AddComponent(id, *transform)
}

func (s *MovementSystem) collidesWithEntities(selfID ecs.Entity, z int, x, y, w, h float64) bool {
	others := ecs.Query[components.PhysicsComponent](s.World)
	for _, otherID := range others {
		if otherID == selfID {
			continue
		}

		proj, _ := ecs.GetComponent[components.ProjectileComponent](s.World, otherID)
		if proj != nil {
			continue // Don't collide with projectiles physically
		}

		otherTrans, _ := ecs.GetComponent[components.TransformComponent](s.World, otherID)

		// Check Z Match
		if otherTrans.Z != z {
			continue
		}

		boxSize := 14.0
		offset := (32.0 - boxSize) / 2.0
		otherX := otherTrans.X + offset
		otherY := otherTrans.Y + offset

		if s.rectOverlap(x, y, w, h, otherX, otherY, boxSize, boxSize) {
			return true
		}
	}
	return false
}

func (s *MovementSystem) collidesAt(z int, x, y, w, h float64) bool {
	gameMap, ok := s.Maps[z]
	if !ok {
		return true // No map at this Z = Solid Void? Or empty? Better block.
	}

	tileSize := 32.0
	// Check all tiles the box might overlap
	startTX := int(math.Floor(x / tileSize))
	startTY := int(math.Floor(y / tileSize))
	endTX := int(math.Floor((x + w) / tileSize))
	endTY := int(math.Floor((y + h) / tileSize))

	for ty := startTY; ty <= endTY; ty++ {
		for tx := startTX; tx <= endTX; tx++ {
			if tx < 0 || tx >= gameMap.Width || ty < 0 || ty >= gameMap.Height {
				return true // Out of bounds is a collision
			}

			tile := gameMap.Tiles[ty][tx]
			if s.isTileSolid(tile, tx, ty, x, y, w, h) {
				return true
			}

			// Check Objects Layer (Trees)
			objID := gameMap.Objects[ty][tx]
			if objID > 0 { // Any object > 0 is solid for now (Trees mostly)
				// Treat as Tree
				// Assuming all objects are trees for now or centered obstructions
				treeSize := 16.0
				offset := (tileSize - treeSize) / 2.0
				obsX := float64(tx)*tileSize + offset
				obsY := float64(ty)*tileSize + offset
				if s.rectOverlap(x, y, w, h, obsX, obsY, treeSize, treeSize) {
					return true
				}
			}
		}
	}

	return false
}

func (s *MovementSystem) isTileSolid(tile world.Tile, tx, ty int, x, y, w, h float64) bool {
	tileSize := 32.0
	tileX := float64(tx) * tileSize
	tileY := float64(ty) * tileSize

	localX := x - tileX
	localY := y - tileY

	// General Solid Check
	if tile.Type.IsSolid() {
		// Special handling for partial solids (Edges/Corners)
		// For now, let's simplify: if it claims to be solid, treat full tile as solid
		// UNLESS we want to keep the sub-tile precision for edges.

		switch tile.Type {
		case world.TileWaterEdgeTop:
			return localY+h > 16
		case world.TileWaterEdgeBottom:
			return localY < 16
		case world.TileWaterEdgeLeft:
			return localX+w > 16
		case world.TileWaterEdgeRight:
			return localX < 16
		case world.TileWaterCornerTL:
			return localX+w > 16 && localY+h > 16
		case world.TileWaterCornerTR:
			return localX < 16 && localY+h > 16
		case world.TileWaterCornerBL:
			return localX+w > 16 && localY < 16
		case world.TileWaterCornerBR:
			return localX < 16 && localY < 16
		case world.TileTree:
			treeSize := 16.0
			treeOffset := (tileSize - treeSize) / 2.0
			return s.rectOverlap(localX, localY, w, h, treeOffset, treeOffset, treeSize, treeSize)
		default:
			return true // Full solid (Deep Water, Lava, etc)
		}
	}

	return false
}

func (s *MovementSystem) rectOverlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}
