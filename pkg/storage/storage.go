package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DataDir = "data/players"

type PlayerSaveData struct {
	Username       string
	Password       string // Plaintext for now as requested (TODO: Hash)
	X, Y           float64
	Health         float64
	Keybindings    map[string]int  // Action -> Ebiten Key ID
	DebugSettings  map[string]bool // Toggle -> Enabled
	Inventory      []InventorySlotSave
	Hotbar         [10]HotbarSlotSave
	Equipment      [9]EquipmentSlotSave
	UnlockedSpells []string
	OpenMenus      map[string]bool // WindowName -> IsVisible
	IsRunning      bool
}

type InventorySlotSave struct {
	Index    int
	ItemID   string
	Quantity int
}

type HotbarSlotSave struct {
	Type  string
	RefID string
}

type EquipmentSlotSave struct {
	ItemID string
}

func GetFilePath(username string) string {
	return filepath.Join(DataDir, username+".json")
}

func SavePlayer(data PlayerSaveData) error {
	// Ensure dir exists
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return err
	}

	file, err := os.Create(GetFilePath(data.Username))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func LoadPlayer(username string) (*PlayerSaveData, error) {
	file, err := os.Open(GetFilePath(username))
	if err != nil {
		// If file doesn't exist, return nil, nil (not an error, just new player)
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var data PlayerSaveData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}
