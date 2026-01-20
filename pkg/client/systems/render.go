package systems

import (
	"image/color"

	"henry/pkg/network"
	"henry/pkg/shared/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type RenderSystem struct {
	Client   *network.NetworkClient
	UISystem *UISystem // Use UISystem

	// Health Tracking for Dynamic Bars
	HealthTrackers map[uint64]*HealthTracker
}

type HealthTracker struct {
	LastHealth  float64
	CombatTimer float64 // Seconds
}

func NewRenderSystem(client *network.NetworkClient, uiSystem *UISystem) *RenderSystem {
	return &RenderSystem{
		Client:         client,
		UISystem:       uiSystem,
		HealthTrackers: make(map[uint64]*HealthTracker),
	}
}

func (s *RenderSystem) Draw(screen *ebiten.Image) {
	state := s.Client.GetState()
	playerID := s.Client.PlayerEntityID

	var camX, camY float64
	// Find player transform for camera
	for _, entity := range state.Entities {
		if entity.ID == playerID && entity.Transform != nil {
			camX = entity.Transform.X - 400 + 16
			camY = entity.Transform.Y - 300 + 16
			break
		}
	}

	// Draw Map
	var width, height int
	if s.Client.WorldMap != nil {
		width = s.Client.WorldMap.Width
		height = s.Client.WorldMap.Height
	} else {
		// Fallback to packet (should rarely happen if initialized correctly)
		m := s.Client.GetMap()
		width = m.Width
		height = m.Height
	}
	tileSize := 32.0

	if width > 0 {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				tx := float64(x) * tileSize
				ty := float64(y) * tileSize
				if tx+tileSize < camX || tx > camX+800 || ty+tileSize < camY || ty > camY+600 {
					continue
				}

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
					c = color.RGBA{34, 139, 34, 255} // Forest Green
				case world.TileGrassFlowers:
					c = color.RGBA{50, 205, 50, 255} // Lime Green with hypothetical flowers
				case world.TileWater, world.TileWaterShallow:
					c = color.RGBA{0, 191, 255, 255} // Deep Sky Blue
				case world.TileWaterDeep:
					c = color.RGBA{0, 0, 139, 255} // Dark Blue
				case world.TileSand:
					c = color.RGBA{238, 214, 175, 255} // Sand
				case world.TileDirtPath:
					c = color.RGBA{139, 69, 19, 255} // Saddle Brown
				case world.TileCobblePath:
					c = color.RGBA{128, 128, 128, 255} // Gray
				case world.TileStoneFloor:
					c = color.RGBA{105, 105, 105, 255} // Dim Gray
				case world.TileWoodFloor:
					c = color.RGBA{160, 82, 45, 255} // Sienna
				case world.TileSnow:
					c = color.RGBA{255, 250, 250, 255} // Snow
				case world.TileIce:
					c = color.RGBA{176, 224, 230, 255} // Powder Blue
				case world.TileLava:
					c = color.RGBA{255, 69, 0, 255} // Orange Red
				case world.TileWaterEdgeTop, world.TileWaterEdgeBottom, world.TileWaterEdgeLeft, world.TileWaterEdgeRight,
					world.TileWaterCornerTL, world.TileWaterCornerTR, world.TileWaterCornerBL, world.TileWaterCornerBR:
					c = color.RGBA{65, 105, 225, 255} // Royal Blue (Edge)
				default:
					c = color.RGBA{0, 100, 0, 255} // Fallback Dark Green
				}
				vector.DrawFilledRect(screen, float32(tx-camX), float32(ty-camY), float32(tileSize), float32(tileSize), c, false)

				// 2. Draw Objects Layer (If Present)
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
					// Simple Tree Rendering (Circle or Rect)
					// Brown Trunk + Green Top?
					// Just a rect for now.
					treeColor := color.RGBA{1, 50, 32, 200} // Dark Green, Semi-transparent
					// Draw smaller rect centered
					margin := float32(4)
					vector.DrawFilledRect(screen, float32(tx-camX)+margin, float32(ty-camY)+margin, float32(tileSize)-margin*2, float32(tileSize)-margin*2, treeColor, true)
				}
			}
		}
	}

	// Draw Entities
	for _, entity := range state.Entities {
		if entity.Transform != nil && entity.Sprite != nil {
			x := float64(entity.Transform.X - camX)
			y := float64(entity.Transform.Y - camY)

			// Use Sprite Color
			c := entity.Sprite.Color
			vector.DrawFilledRect(screen, float32(x), float32(y), float32(entity.Sprite.Width), float32(entity.Sprite.Height), c, true)

			// Check/Update Health Tracker
			if entity.Stats != nil {
				tracker, exists := s.HealthTrackers[uint64(entity.ID)]
				if !exists {
					tracker = &HealthTracker{LastHealth: entity.Stats.CurrentHealth, CombatTimer: 0}
					s.HealthTrackers[uint64(entity.ID)] = tracker
				}

				if entity.Stats.CurrentHealth != tracker.LastHealth {
					// Only trigger timer if health changed AND is not full?
					// Or just if we took damage?
					// If we respawn, LastHealth=0, Current=Max.
					// We want to HIDE bar.
					if entity.Stats.CurrentHealth == entity.Stats.MaxHealth {
						tracker.CombatTimer = 0
					} else {
						tracker.CombatTimer = 5.0
					}
					tracker.LastHealth = entity.Stats.CurrentHealth
				}
				// dt approximation inside loop (since Render doesn't pass dt, assume 60fps)
				if tracker.CombatTimer > 0 {
					tracker.CombatTimer -= 1.0 / 60.0
				}

				// Draw Health Bar if Combat Active
				if tracker.CombatTimer > 0 {
					barWidth := float32(32)
					healthPct := float32(entity.Stats.CurrentHealth) / float32(entity.Stats.MaxHealth)
					if healthPct < 0 {
						healthPct = 0
					}
					vector.DrawFilledRect(screen, float32(x), float32(y)-10, barWidth, 5, color.RGBA{50, 50, 50, 255}, true)
					vector.DrawFilledRect(screen, float32(x), float32(y)-10, barWidth*healthPct, 5, color.RGBA{0, 255, 0, 255}, true)
				}
			}
		}
	}

	// Draw UI
	s.UISystem.Draw(screen)
}
