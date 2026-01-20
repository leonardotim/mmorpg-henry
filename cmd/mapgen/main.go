package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"henry/pkg/shared/world"
)

type MapData struct {
	Level    int       `json:"level"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Layers   Layers    `json:"layers"`
	Spawners []Spawner `json:"spawners"`
}

type Layers struct {
	Ground  [][]int `json:"ground"`
	Objects [][]int `json:"objects"`
}

type Spawner struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	CharacterID string  `json:"character_id"`
}

func main() {
	width := 60
	height := 60

	ground := make([][]int, height)
	objects := make([][]int, height)
	for i := range ground {
		ground[i] = make([]int, width)
		objects[i] = make([]int, width)
	}

	// Perlin-ish noise / Biome logic simulation
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Base: Grass
			ground[y][x] = int(world.TileGrass)

			// Center Lake (Deep & Shallow)
			cx, cy := 30, 30
			dx, dy := x-cx, y-cy
			distSq := dx*dx + dy*dy

			if distSq < 15 {
				ground[y][x] = int(world.TileWaterDeep)
			} else if distSq < 60 {
				ground[y][x] = int(world.TileWaterShallow)
			} else if distSq < 100 {
				ground[y][x] = int(world.TileSand) // Beach
			} else {
				// Random Biomes
				rn := rand.Intn(100)
				if rn < 5 {
					ground[y][x] = int(world.TileGrassFlowers)
				} else if rn > 90 {
					// Forest patch (handled by objects logic below, but maybe dirt ground?)
				}
			}
		}
	}

	// Paths: Cross from W->E and N->S
	for i := 0; i < width; i++ {
		// Horizontal Path
		if i < 20 || i > 40 { // Don't bridge the lake automatically, let's make a dock/bridge
			ground[30][i] = int(world.TileCobblePath)
		} else {
			// Bridge over lake
			ground[30][i] = int(world.TileCobblePath) // Wooden bridge in future?
		}

		// Vertical Path
		ground[i][30] = int(world.TileDirtPath)
	}

	// Objects (Trees)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t := world.TileType(ground[y][x])

			// Trees only on Grass
			if t == world.TileGrass || t == world.TileGrassFlowers {
				if rand.Float64() < 0.1 { // 10% density
					objects[y][x] = int(world.TileTree) // Tree ID
				}
			}
		}
	}

	// Spawners
	spawners := []Spawner{
		{X: 100, Y: 100, CharacterID: "guard_melee"},
		{X: 150, Y: 100, CharacterID: "guard_melee"},
		{X: 500, Y: 500, CharacterID: "guard_ranged"},
	}

	// Add random NPCs
	for i := 0; i < 20; i++ {
		var sx, sy float64
		valid := false

		// Try 10 times to find a valid spot
		for attempt := 0; attempt < 10; attempt++ {
			sx = 200 + rand.Float64()*1000.0
			sy = 200 + rand.Float64()*1000.0

			if sx > float64(width)*32-100 {
				sx -= 200
			}
			if sy > float64(height)*32-100 {
				sy -= 200
			}

			// Check full bounding box (32x32)
			// Corners: TL, TR, BL, BR
			corners := [][2]float64{
				{sx, sy},
				{sx + 31, sy},
				{sx, sy + 31},
				{sx + 31, sy + 31},
			}

			valid = true
			for _, c := range corners {
				cx, cy := int(c[0]/32.0), int(c[1]/32.0)
				if cx < 0 || cx >= width || cy < 0 || cy >= height {
					valid = false
					break
				}
				if world.TileType(ground[cy][cx]).IsSolid() {
					valid = false
					break
				}
				if objects[cy][cx] > 0 {
					valid = false
					break
				}
			}

			if valid {
				break
			}
		}

		if !valid {
			continue // Skip this one
		}

		charType := "guard_melee"
		if rand.Float64() < 0.3 {
			charType = "guard_ranged"
		}

		spawners = append(spawners, Spawner{
			X:           sx,
			Y:           sy,
			CharacterID: charType,
		})
	}

	output := MapData{
		Level:  0,
		Width:  width,
		Height: height,
		Layers: Layers{
			Ground:  ground,
			Objects: objects,
		},
		Spawners: spawners,
	}

	file, _ := json.MarshalIndent(output, "", "  ")
	os.WriteFile("data/maps/level_0.json", file, 0644)
	fmt.Println("Generated level_0.json")
}
