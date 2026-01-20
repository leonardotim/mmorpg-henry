package systems

import (
	"henry/pkg/items"
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
	"henry/pkg/shared/world"
	"math"
	"math/rand"
)

type AISystem struct {
	World *ecs.World
	Maps  map[int]*world.Map
}

func NewAISystem(world *ecs.World, maps map[int]*world.Map) *AISystem {
	return &AISystem{
		World: world,
		Maps:  maps,
	}
}

func (s *AISystem) Update(dt float64) {
	entities := ecs.Query[components.AIComponent](s.World)

	for _, id := range entities {
		ai, _ := ecs.GetComponent[components.AIComponent](s.World, id)
		input, _ := ecs.GetComponent[components.InputComponent](s.World, id)
		transform, _ := ecs.GetComponent[components.TransformComponent](s.World, id)

		if ai == nil || input == nil || transform == nil {
			continue
		}

		currentMap, ok := s.Maps[transform.Z]
		if !ok {
			continue // No map for this entity?
		}

		// Reset Inputs Frame
		input.Up = false
		input.Down = false
		input.Left = false
		input.Right = false
		input.Attack = false

		// Check Target Validity
		if ai.TargetID != 0 {
			targetTrans, _ := ecs.GetComponent[components.TransformComponent](s.World, ai.TargetID)
			if targetTrans == nil || targetTrans.Z != transform.Z { // Verify Target is on same Z
				// Target dead or gone or different level
				ai.TargetID = 0
				ai.State = "wander"
			} else {
				// Use Dynamic Center
				selfX, selfY := s.getEntityCenter(id)
				targetX, targetY := s.getEntityCenter(ai.TargetID)

				// Dist Logic (Center to Center)
				dx := targetX - selfX
				dy := targetY - selfY
				dist := math.Sqrt(dx*dx + dy*dy)

				// Face Target (Input uses Transform, maybe update to Center too? Client handles offsets usually)
				// Keeping input raw for now, or use target center.
				input.MouseX = targetX
				input.MouseY = targetY

				// Use Multi-Ray LOS (Function adds offsets internally)
				hasLOS := s.HasLineOfSight(currentMap, selfX, selfY, targetX, targetY)

				// Determine Attack Range from Equipment
				attackRange := 50.0 // Default Melee
				weaponType := "melee"
				if equip, ok := ecs.GetComponent[components.EquipmentComponent](s.World, id); ok {
					if weaponID := equip.Slots[components.SlotWeapon].ItemID; weaponID != "" {
						if item, exists := items.Get(weaponID); exists && item.WeaponStats != nil {
							attackRange = item.WeaponStats.Range
							if attackRange > 60 {
								weaponType = "ranged"
							}
							attackRange *= 0.8
						}
					}
				}

				// Attack Logic
				// Ranged: Needs Range AND LOS
				// Melee: Needs Range (LOS implied by close range usually, but strictly required for corners)
				canAttack := dist <= attackRange
				if weaponType == "ranged" && !hasLOS {
					canAttack = false // Can't shoot through walls
				}

				// LEASH CHECK (Global for TargetID != 0)
				// Must override attack logic to prevent infinite kiting/stuck rangers
				dxSpawn := transform.X - ai.SpawnX
				dySpawn := transform.Y - ai.SpawnY
				if dxSpawn*dxSpawn+dySpawn*dySpawn > ai.LeashRange*ai.LeashRange {
					// Too far! Go home.
					ai.State = "return"
					ai.TargetID = 0
					ai.Path = nil // Reset path
					// log.Printf("Entity %d Leashed! Pos: %.1f,%.1f Spawn: %.1f,%.1f DistSq: %.1f",
					// 	id, transform.X, transform.Y, ai.SpawnX, ai.SpawnY, dxSpawn*dxSpawn+dySpawn*dySpawn)

					// Critical: Save state before returning!
					s.World.AddComponent(id, *ai)
					s.World.AddComponent(id, *input)
					return // Skip rest of frame
				}

				if canAttack {
					// ATTACK
					ai.State = "attack"
					input.Attack = true
				} else {
					// CHASE
					ai.State = "chase"
					ai.PathTimer -= dt

					var moveTargetX, moveTargetY float64

					if hasLOS {
						// Direct Chase - Clear path data
						ai.Path = nil
						moveTargetX = targetTrans.X
						moveTargetY = targetTrans.Y
					} else {
						// Blocked! Pathfind

						// Recalculate path if timer expired or no path
						if ai.PathTimer <= 0 || len(ai.Path) == 0 {
							// Calculate new path
							ai.Path = s.FindPath(currentMap, selfX, selfY, targetX, targetY)
							ai.PathTimer = 0.5 // Refresh path every 0.5s to track moving target
						}

						// Follow Path
						if len(ai.Path) > 0 {
							moveTargetX = ai.Path[0][0]
							moveTargetY = ai.Path[0][1]

							// Check if reached node (within 10px)
							dx := moveTargetX - transform.X
							dy := moveTargetY - transform.Y
							if dx*dx+dy*dy < 100.0 {
								// Node reached, advance
								ai.Path = ai.Path[1:]
								if len(ai.Path) > 0 {
									moveTargetX = ai.Path[0][0]
									moveTargetY = ai.Path[0][1]
								}
							}
						} else {
							// No path found? Direct chase as failover
							moveTargetX = targetTrans.X
							moveTargetY = targetTrans.Y
						}
					}

					// Calculate Vector to MoveTarget
					dx = moveTargetX - transform.X
					dy = moveTargetY - transform.Y
					distToNode := math.Sqrt(dx*dx + dy*dy)

					if distToNode > 0 {
						dx /= distToNode
						dy /= distToNode
					}

					// Apply Movement Inputs
					if math.Abs(dx) > math.Abs(dy) {
						if dx > 0 {
							input.Right = true
						} else {
							input.Left = true
						}
						// Smoothing
						if dy > 0.5 {
							input.Down = true
						} else if dy < -0.5 {
							input.Up = true
						}
					} else {
						if dy > 0 {
							input.Down = true
						} else {
							input.Up = true
						}
						if dx > 0.5 {
							input.Right = true
						} else if dx < -0.5 {
							input.Left = true
						}
					}
				}
			}
		} else if ai.State == "return" {
			// RETURNING HOME
			dx := ai.SpawnX - transform.X
			dy := ai.SpawnY - transform.Y
			distSq := dx*dx + dy*dy

			// Safely back within range? (e.g. within 50px of spawn)
			// This prevents them from walking ALL the way back to the exact pixel
			if distSq < 50*50 {
				// Home reached (enough)
				ai.State = "wander"
				ai.StateTimer = 2.0 // Chill for a bit
			} else {
				// Move towards home
				// Simple direct movement for now, improve with pathfinding if needed
				// Actually, should reuse pathfinding to avoid getting stuck on return
				ai.PathTimer -= dt
				if ai.PathTimer <= 0 || len(ai.Path) == 0 {
					ai.Path = s.FindPath(currentMap, transform.X, transform.Y, ai.SpawnX, ai.SpawnY)
					ai.PathTimer = 1.0
					// log.Printf("NPC %d Returning. Pos: %.1f,%.1f -> Spawn: %.1f,%.1f. DistSq: %.1f, PathLen: %d",
					// 	id, transform.X, transform.Y, ai.SpawnX, ai.SpawnY, distSq, pathLen)
				}

				var moveTargetX, moveTargetY float64
				if len(ai.Path) > 0 {
					moveTargetX = ai.Path[0][0]
					moveTargetY = ai.Path[0][1]

					mdx := moveTargetX - transform.X
					mdy := moveTargetY - transform.Y
					// Increase tolerance to avoid orbiting (Speed is ~6, so < 16 was too tight)
					if mdx*mdx+mdy*mdy < 100.0 { // < 10px distance
						ai.Path = ai.Path[1:]
						if len(ai.Path) > 0 {
							moveTargetX = ai.Path[0][0]
							moveTargetY = ai.Path[0][1]
						}
					}
				} else {
					// Fallback: Direct line
					moveTargetX = ai.SpawnX
					moveTargetY = ai.SpawnY
				}

				// Move Logic
				finalDx := moveTargetX - transform.X
				finalDy := moveTargetY - transform.Y
				distFinal := math.Sqrt(finalDx*finalDx + finalDy*finalDy)
				if distFinal > 0 {
					finalDx /= distFinal
					finalDy /= distFinal
				}

				if math.Abs(finalDx) > math.Abs(finalDy) {
					if finalDx > 0 {
						input.Right = true
					} else {
						input.Left = true
					}
				} else {
					if finalDy > 0 {
						input.Down = true
					} else {
						input.Up = true
					}
				}
			}

		} else {
			// Wander Logic

			// LEASH CHECK (Wander)
			dxSpawn := transform.X - ai.SpawnX
			dySpawn := transform.Y - ai.SpawnY
			if dxSpawn*dxSpawn+dySpawn*dySpawn > ai.LeashRange*ai.LeashRange {
				ai.State = "return"
				ai.TargetID = 0
				ai.Path = nil
			} else {
				ai.StateTimer -= dt
				if ai.StateTimer <= 0 {
					s.pickNewState(ai)
				}
				s.applyWanderState(ai, input, transform)
			}
		}

		// Save components back
		s.World.AddComponent(id, *ai)
		s.World.AddComponent(id, *input)
	}
}

func (s *AISystem) pickNewState(ai *components.AIComponent) {
	// 50% chance to idle, 50% chance to move
	if rand.Float64() < 0.5 {
		ai.State = "idle"
		ai.StateTimer = 1.0 + rand.Float64()*2.0 // Idle for 1-3 seconds
	} else {
		ai.State = "move"
		ai.StateTimer = 1.0 + rand.Float64()*2.0 // Move for 1-3 seconds
		ai.MoveDirection = rand.Intn(4)          // 0-3 direction
	}
}

func (s *AISystem) applyWanderState(ai *components.AIComponent, input *components.InputComponent, transform *components.TransformComponent) {
	if ai.State == "move" {
		switch ai.MoveDirection {
		case 0: // Up
			input.Up = true
			input.MouseX = transform.X
			input.MouseY = transform.Y - 100
		case 1: // Down
			input.Down = true
			input.MouseX = transform.X
			input.MouseY = transform.Y + 100
		case 2: // Left
			input.Left = true
			input.MouseX = transform.X - 100
			input.MouseY = transform.Y
		case 3: // Right
			input.Right = true
			input.MouseX = transform.X + 100
			input.MouseY = transform.Y
		}
	}
}

// getEntityCenter calculates the visual center of an entity
func (s *AISystem) getEntityCenter(id ecs.Entity) (float64, float64) {
	trans, _ := ecs.GetComponent[components.TransformComponent](s.World, id)
	if trans == nil {
		return 0, 0
	}
	w, h := 32.0, 32.0
	if sprite, ok := ecs.GetComponent[components.SpriteComponent](s.World, id); ok {
		w = float64(sprite.Width)
		h = float64(sprite.Height)
	}
	return trans.X + w/2, trans.Y + h/2
}

// HasLineOfSight checks if a straight line between start and end is clear of obstacles
// Checks multiple rays to ensure the entity's width allows passage
func (s *AISystem) HasLineOfSight(m *world.Map, x1, y1, x2, y2 float64) bool {
	// Offsets for approx 32x32 entity
	// We check the Center and the 4 corners (shrunk slightly to 24x24 box to avoid grazing)
	offsets := [][2]float64{
		{0, 0},     // Center
		{-12, -12}, // TL
		{12, -12},  // TR
		{-12, 12},  // BL
		{12, 12},   // BR
	}

	for _, off := range offsets {
		start := [2]float64{x1 + off[0], y1 + off[1]}
		end := [2]float64{x2 + off[0], y2 + off[1]}

		if !s.castRay(m, start[0], start[1], end[0], end[1]) {
			return false
		}
	}
	return true
}

func (s *AISystem) castRay(m *world.Map, x1, y1, x2, y2 float64) bool {
	dist := math.Sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1))
	steps := int(dist / 8.0) // Check every 8 pixels

	if steps == 0 {
		return true
	}

	dx := (x2 - x1) / float64(steps)
	dy := (y2 - y1) / float64(steps)

	cx, cy := x1, y1
	for i := 0; i < steps; i++ {
		cx += dx
		cy += dy

		tx := int(cx / 32.0)
		ty := int(cy / 32.0)
		if tx >= 0 && tx < m.Width && ty >= 0 && ty < m.Height {
			tile := m.Tiles[ty][tx]
			if tile.Type.IsSolid() {
				return false
			}
			if m.Objects[ty][tx] > 0 {
				return false
			}
		}
	}
	return true
}

type Node struct {
	X, Y    int
	G, H, F float64
	Parent  *Node
}

// FindPath finds a path from start to end using A* Algorithm
func (s *AISystem) FindPath(m *world.Map, startX, startY, endX, endY float64) [][]float64 {
	// Grid Coordinates
	startTX := int((startX + 16) / 32.0)
	startTY := int((startY + 16) / 32.0)
	endTX := int((endX + 16) / 32.0)
	endTY := int((endY + 16) / 32.0)

	if startTX == endTX && startTY == endTY {
		return nil
	}

	// Bounds check target
	if endTX < 0 || endTX >= m.Width || endTY < 0 || endTY >= m.Height {
		return nil
	}
	// Target blockage check (Basic)
	if m.Tiles[endTY][endTX].Type.IsSolid() || m.Objects[endTY][endTX] > 0 {
		return nil
	}

	openList := make(map[int]*Node)
	closedList := make(map[int]bool)

	startNode := &Node{X: startTX, Y: startTY, G: 0, H: 0, F: 0}
	openList[startTY*m.Width+startTX] = startNode

	var finalNode *Node

	// Directions: Cardinal + Diagonal
	// Up, Down, Left, Right, TL, TR, BL, BR
	dirs := [][2]int{
		{0, -1}, {0, 1}, {-1, 0}, {1, 0},
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1},
	}

	for len(openList) > 0 {
		// Get node with lowest F
		var curr *Node
		var currIdx int
		minF := math.MaxFloat64

		for idx, node := range openList {
			if node.F < minF {
				minF = node.F
				curr = node
				currIdx = idx
			}
		}

		delete(openList, currIdx)
		closedList[currIdx] = true

		// Found Target?
		if curr.X == endTX && curr.Y == endTY {
			finalNode = curr
			break
		}

		// Neighbors
		for i, d := range dirs {
			nx, ny := curr.X+d[0], curr.Y+d[1]

			// Bounds
			if nx < 0 || nx >= m.Width || ny < 0 || ny >= m.Height {
				continue
			}

			idx := ny*m.Width + nx
			if closedList[idx] {
				continue
			}

			// Collision Check
			if m.Tiles[ny][nx].Type.IsSolid() || m.Objects[ny][nx] > 0 {
				continue
			}

			// Diagonal Safety: Check adjacent cardinals
			// If moving diagonally, both cardinals must be free to avoid cutting corners
			if i >= 4 { // Diagonals are indices 4-7
				// e.g., TL (-1, -1) needs (-1, 0) and (0, -1) free
				c1x, c1y := curr.X+d[0], curr.Y
				c2x, c2y := curr.X, curr.Y+d[1]

				// Using simple existence checks - improve if strict validation needed
				blocked := false
				if c1x >= 0 && c1x < m.Width && c1y >= 0 && c1y < m.Height {
					if m.Tiles[c1y][c1x].Type.IsSolid() || m.Objects[c1y][c1x] > 0 {
						blocked = true
					}
				}
				if c2x >= 0 && c2x < m.Width && c2y >= 0 && c2y < m.Height {
					if m.Tiles[c2y][c2x].Type.IsSolid() || m.Objects[c2y][c2x] > 0 {
						blocked = true
					}
				}
				if blocked {
					continue
				}
			}

			// Costs
			moveCost := 1.0
			if i >= 4 {
				moveCost = 1.414 // Sqrt(2) for diagonals
			}

			gScore := curr.G + moveCost
			hScore := math.Sqrt(float64((nx-endTX)*(nx-endTX) + (ny-endTY)*(ny-endTY))) // Euclidean
			fScore := gScore + hScore

			if existing, exists := openList[idx]; exists {
				if gScore < existing.G {
					existing.G = gScore
					existing.F = fScore
					existing.Parent = curr
				}
			} else {
				node := &Node{X: nx, Y: ny, G: gScore, H: hScore, F: fScore, Parent: curr}
				openList[idx] = node
			}
		}
	}

	if finalNode != nil {
		// Reconstruct Path
		var rawPath [][]float64
		curr := finalNode
		for curr != nil {
			// Center of tile
			rawPath = append([][]float64{{float64(curr.X)*32 + 16, float64(curr.Y)*32 + 16}}, rawPath...)
			curr = curr.Parent
		}

		// String Pulling (Smoothing)
		if len(rawPath) > 2 {
			return s.stringPull(m, rawPath)
		}
		if len(rawPath) > 1 {
			return rawPath[1:] // Skip start node
		}
		return rawPath
	}

	return nil
}

// stringPull optimizes the path by removing unnecessary nodes
func (s *AISystem) stringPull(m *world.Map, path [][]float64) [][]float64 {
	if len(path) < 3 {
		return path
	}

	smoothPath := [][]float64{path[0]}
	currIdx := 0

	for currIdx < len(path)-1 {
		// Look ahead as far as possible
		nextIdx := currIdx + 1
		for i := len(path) - 1; i > currIdx+1; i-- {
			if s.HasLineOfSight(m, path[currIdx][0], path[currIdx][1], path[i][0], path[i][1]) {
				nextIdx = i
				break
			}
		}
		smoothPath = append(smoothPath, path[nextIdx])
		currIdx = nextIdx
	}

	if len(smoothPath) > 1 {
		return smoothPath[1:] // Return next steps (exclude current/start pos)
	}
	return smoothPath
}
