package systems

import (
	"image/color"
	"math"

	"henry/pkg/client/assets"
	"henry/pkg/network"
	"henry/pkg/shared/config"
	"henry/pkg/shared/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type RenderSystem struct {
	Client   *network.NetworkClient
	UISystem *UISystem // Use UISystem

	// Health Tracking for Dynamic Bars
	HealthTrackers    map[uint64]*HealthTracker
	AnimationTrackers map[uint64]*AnimationTracker
}

type HealthTracker struct {
	LastHealth  float64
	CombatTimer float64 // Seconds
}

type AnimationTracker struct {
	CurrentAnimation string
	FrameIndex       int
	Timer            float64
	LastX, LastY     float64
	MoveDecayTimer   float64
	IsMoving         bool
}

func NewRenderSystem(client *network.NetworkClient, uiSystem *UISystem) *RenderSystem {
	return &RenderSystem{
		Client:            client,
		UISystem:          uiSystem,
		HealthTrackers:    make(map[uint64]*HealthTracker),
		AnimationTrackers: make(map[uint64]*AnimationTracker),
	}
}

func (s *RenderSystem) Draw(screen *ebiten.Image) {
	state := s.Client.GetState()
	playerID := s.Client.PlayerEntityID

	tileSize := float64(config.TileSize) // Should be 64.0

	var camX, camY float64
	// Find player transform for camera
	for _, entity := range state.Entities {
		if entity.ID == playerID && entity.Transform != nil {
			camX = entity.Transform.X - 400 + tileSize/2
			camY = entity.Transform.Y - 300 + tileSize/2
			break
		}
	}

	// Draw Map
	var width, height int
	if s.Client.WorldMap != nil {
		width = s.Client.WorldMap.Width
		height = s.Client.WorldMap.Height
	} else {
		m := s.Client.GetMap()
		width = m.Width
		height = m.Height
	}

	if width > 0 {
		startX := int(math.Floor((camX - 800) / tileSize))
		startY := int(math.Floor((camY - 600) / tileSize))
		endX := int(math.Ceil((camX + 800) / tileSize))
		endY := int(math.Ceil((camY + 600) / tileSize))

		// Bounds Clamp
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}
		if endX > width {
			endX = width
		}
		if endY > height {
			endY = height
		}

		for y := startY; y < endY; y++ {
			for x := startX; x < endX; x++ {
				tx := float64(x) * tileSize
				ty := float64(y) * tileSize

				// 1. Draw Ground Layer
				var c color.Color
				var tileType world.TileType

				if s.Client.WorldMap != nil {
					tileType = s.Client.WorldMap.Tiles[y][x].Type
				} else {
					m := s.Client.GetMap()
					if len(m.Tiles) > y*width+x {
						tileType = world.TileType(m.Tiles[y*width+x])
					}
				}

				switch tileType {
				case world.TileGrass:
					c = color.RGBA{34, 139, 34, 255}
				case world.TileGrassFlowers:
					c = color.RGBA{50, 205, 50, 255}
				case world.TileWater, world.TileWaterShallow:
					c = color.RGBA{0, 191, 255, 255}
				case world.TileWaterDeep:
					c = color.RGBA{0, 0, 139, 255}
				case world.TileSand:
					c = color.RGBA{238, 214, 175, 255}
				case world.TileDirtPath:
					c = color.RGBA{139, 69, 19, 255}
				case world.TileCobblePath:
					c = color.RGBA{128, 128, 128, 255}
				case world.TileStoneFloor:
					c = color.RGBA{105, 105, 105, 255}
				case world.TileWoodFloor:
					c = color.RGBA{160, 82, 45, 255}
				case world.TileSnow:
					c = color.RGBA{255, 250, 250, 255}
				case world.TileIce:
					c = color.RGBA{176, 224, 230, 255}
				case world.TileLava:
					c = color.RGBA{255, 69, 0, 255}
				default:
					c = color.RGBA{0, 100, 0, 255} // Fallback
				}
				// Draw Rect
				vector.DrawFilledRect(screen, float32(tx-camX), float32(ty-camY), float32(tileSize), float32(tileSize), c, false)

				// 2. Draw Objects Layer
				var obj int
				if s.Client.WorldMap != nil {
					if y < len(s.Client.WorldMap.Objects) && x < len(s.Client.WorldMap.Objects[y]) {
						obj = s.Client.WorldMap.Objects[y][x]
					}
				} else {
					m := s.Client.GetMap()
					if len(m.Objects) > y*width+x {
						obj = m.Objects[y*width+x]
					}
				}

				if obj > 0 {
					treeColor := color.RGBA{1, 50, 32, 200}
					margin := float32(tileSize * 0.1)
					vector.DrawFilledRect(screen, float32(tx-camX)+margin, float32(ty-camY)+margin, float32(tileSize)-margin*2, float32(tileSize)-margin*2, treeColor, true)
				}
			}
		}
	}

	dt := 1.0 / 60.0

	// Draw Entities
	for _, entity := range state.Entities {
		if entity.Transform != nil {
			x := float64(entity.Transform.X - camX)
			y := float64(entity.Transform.Y - camY)

			var spriteDrawn bool

			// Determine Character Type (From Component)
			charName := ""
			if entity.Sprite != nil {
				charName = entity.Sprite.CharType
			}

			if charName != "" {
				// DRAW ANIMATED CHARACTER
				// Update Animation Tracker
				tracker, exists := s.AnimationTrackers[uint64(entity.ID)]
				if !exists {
					tracker = &AnimationTracker{LastX: entity.Transform.X, LastY: entity.Transform.Y}
					s.AnimationTrackers[uint64(entity.ID)] = tracker
				}

				// Motion Check (Squared Distance)
				dx := entity.Transform.X - tracker.LastX
				dy := entity.Transform.Y - tracker.LastY
				distSq := dx*dx + dy*dy

				if distSq > 0.01 {
					tracker.IsMoving = true
					tracker.MoveDecayTimer = 0.2
				} else {
					tracker.MoveDecayTimer -= dt
					if tracker.MoveDecayTimer <= 0 {
						tracker.IsMoving = false
					}
				}

				tracker.LastX = entity.Transform.X
				tracker.LastY = entity.Transform.Y

				desiredAnim := "breathing-idle"
				if tracker.IsMoving {
					desiredAnim = "walk"
				}

				if tracker.CurrentAnimation != desiredAnim {
					tracker.CurrentAnimation = desiredAnim
					tracker.FrameIndex = 0
					tracker.Timer = 0
				}

				// Advance Frame
				tracker.Timer += dt
				frameDuration := 0.1
				if tracker.Timer >= frameDuration {
					tracker.Timer = 0
					tracker.FrameIndex++
				}

				// Determine Direction
				direction := getDirectionFromAngle(entity.Transform.Rotation)

				// Get Frame
				img := assets.GetCharacterFrame(charName, tracker.CurrentAnimation, direction, tracker.FrameIndex)
				if img != nil {
					opts := &ebiten.DrawImageOptions{}
					// Centering Logic for 64x64 Tile
					// Sprite 56x56
					// Offset = (64 - 56) / 2 = 4
					opts.GeoM.Translate(x+4, y+4)
					screen.DrawImage(img, opts)
					spriteDrawn = true
				}
			} else if entity.Sprite != nil && entity.Sprite.Texture != "" {
				// DRAW TEXTURED PROJECTILE
				projImg := assets.GetImage(entity.Sprite.Texture)
				if projImg != nil {
					opts := &ebiten.DrawImageOptions{}
					w, h := projImg.Bounds().Dx(), projImg.Bounds().Dy()

					// 1. Center the rotation (translate to -center)
					opts.GeoM.Translate(-float64(w)/2, -float64(h)/2)
					// 2. Rotate
					opts.GeoM.Rotate(entity.Transform.Rotation)
					// 3. Translate to world position (centered)
					opts.GeoM.Translate(x+float64(w)/2, y+float64(h)/2)

					screen.DrawImage(projImg, opts)
					spriteDrawn = true
				}
			}

			// Fallback
			if !spriteDrawn && entity.Sprite != nil {
				c := entity.Sprite.Color
				vector.DrawFilledRect(screen, float32(x), float32(y), float32(entity.Sprite.Width), float32(entity.Sprite.Height), c, true)
			}

			// Health Bar
			if entity.Stats != nil {
				tracker, exists := s.HealthTrackers[uint64(entity.ID)]
				if !exists {
					tracker = &HealthTracker{LastHealth: entity.Stats.CurrentHealth, CombatTimer: 0}
					s.HealthTrackers[uint64(entity.ID)] = tracker
				}

				if entity.Stats.CurrentHealth != tracker.LastHealth {
					if entity.Stats.CurrentHealth == entity.Stats.MaxHealth {
						tracker.CombatTimer = 0
					} else {
						tracker.CombatTimer = 5.0
					}
					tracker.LastHealth = entity.Stats.CurrentHealth
				}
				if tracker.CombatTimer > 0 {
					tracker.CombatTimer -= dt
				}

				if tracker.CombatTimer > 0 {
					barWidth := float32(32)
					healthPct := float32(entity.Stats.CurrentHealth) / float32(entity.Stats.MaxHealth)
					if healthPct < 0 {
						healthPct = 0
					}

					// Center Bar: Tile(64) - Bar(32) / 2 = 16
					barX := float32(x) + 16

					vector.DrawFilledRect(screen, barX, float32(y)-10, barWidth, 5, color.RGBA{50, 50, 50, 255}, true)
					vector.DrawFilledRect(screen, barX, float32(y)-10, barWidth*healthPct, 5, color.RGBA{0, 255, 0, 255}, true)
				}
			}
		}
	}

	// Draw UI
	s.UISystem.Draw(screen)
}

func getDirectionFromAngle(angle float64) string {
	// angle is radians.
	// math.Atan2 returns -PI to PI.

	deg := angle * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	// Now 0..360. 0=East, 90=South, 180=West, 270=North.

	// Offset by 22.5 to make integer division easy
	index := int((deg+22.5)/45.0) % 8
	// 0 = East
	// 1 = South-East
	// 2 = South
	// 3 = South-West
	// 4 = West
	// 5 = North-West
	// 6 = North
	// 7 = North-East

	dirs := []string{"east", "south-east", "south", "south-west", "west", "north-west", "north", "north-east"}
	return dirs[index]
}
