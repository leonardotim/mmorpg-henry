package world

import (
	"encoding/json"
	"fmt"
	"os"
)

type MapDefinition struct {
	Level    int          `json:"level"`
	Width    int          `json:"width"`
	Height   int          `json:"height"`
	Layers   MapLayers    `json:"layers"`
	Spawners []SpawnerDef `json:"spawners"`
}

type MapLayers struct {
	Ground  [][]int `json:"ground"`
	Objects [][]int `json:"objects"`
}

type SpawnerDef struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	CharacterID string  `json:"character_id"`
}

func LoadMap(path string) (*Map, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var def MapDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse map json: %w", err)
	}

	m := NewMap(def.Width, def.Height)
	m.Level = def.Level

	// Populate Spawners
	for _, s := range def.Spawners {
		m.Spawners = append(m.Spawners, Spawner{
			X:           s.X,
			Y:           s.Y,
			CharacterID: s.CharacterID,
		})
	}

	// Populate Layers
	// Ground
	if len(def.Layers.Ground) == def.Height {
		for y := 0; y < def.Height; y++ {
			if len(def.Layers.Ground[y]) != def.Width {
				fmt.Printf("Warning: Ground layer row %d width mismatch. Expected %d, got %d\n", y, def.Width, len(def.Layers.Ground[y]))
				continue
			}
			for x := 0; x < def.Width; x++ {
				m.Tiles[y][x] = Tile{Type: TileType(def.Layers.Ground[y][x])}
			}
		}
	} else {
		fmt.Printf("Warning: Ground layer height mismatch. Expected %d, got %d\n", def.Height, len(def.Layers.Ground))
	}

	// Objects
	if len(def.Layers.Objects) == def.Height {
		for y := 0; y < def.Height; y++ {
			if len(def.Layers.Objects[y]) != def.Width {
				continue
			}
			for x := 0; x < def.Width; x++ {
				m.Objects[y][x] = def.Layers.Objects[y][x]
			}
		}
	} else {
		// Just leave empty if missing or mismatch
	}

	return m, nil
}
