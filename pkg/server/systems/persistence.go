package systems

import (
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
	"henry/pkg/storage"
	"log"
)

type PersistenceSystem struct {
	World *ecs.World
}

func NewPersistenceSystem(world *ecs.World) *PersistenceSystem {
	return &PersistenceSystem{
		World: world,
	}
}

func (s *PersistenceSystem) SavePlayer(id ecs.Entity, username string) error {
	trans, _ := ecs.GetComponent[components.TransformComponent](s.World, id)
	stats, _ := ecs.GetComponent[components.StatsComponent](s.World, id)

	if trans == nil || stats == nil {
		log.Printf("PersistenceSystem: Skip save for %s - Trans: %v, Stats: %v", username, trans != nil, stats != nil)
		return nil // Nothing to save or incomplete entity
	}

	existing, _ := storage.LoadPlayer(username)
	if existing == nil {
		existing = &storage.PlayerSaveData{Username: username}
	}

	data := storage.PlayerSaveData{
		Username:    username,
		Password:    existing.Password,
		X:           trans.X,
		Y:           trans.Y,
		Health:      stats.CurrentHealth,
		Keybindings: existing.Keybindings,
		OpenMenus:   existing.OpenMenus,
		IsRunning:   existing.IsRunning,
	}

	// Update Keybindings from world component if present
	kb, _ := ecs.GetComponent[components.KeybindingsComponent](s.World, id)
	if kb != nil {
		data.Keybindings = kb.Bindings
	}

	// Update IsRunning from world component if present
	input, _ := ecs.GetComponent[components.InputComponent](s.World, id)
	if input != nil {
		data.IsRunning = input.IsRunning
	}

	// Save Inventory
	inv, _ := ecs.GetComponent[components.InventoryComponent](s.World, id)
	if inv != nil {
		saveSlots := make([]storage.InventorySlotSave, 0)
		for i, slot := range inv.Slots {
			if slot.ItemID != "" && slot.Quantity > 0 {
				saveSlots = append(saveSlots, storage.InventorySlotSave{
					Index:    i,
					ItemID:   slot.ItemID,
					Quantity: slot.Quantity,
				})
			}
		}
		data.Inventory = saveSlots
	}

	// Save Hotbar
	hotbar, _ := ecs.GetComponent[components.HotbarComponent](s.World, id)
	if hotbar != nil {
		var saveHotbar [10]storage.HotbarSlotSave
		for i, slot := range hotbar.Slots {
			saveHotbar[i] = storage.HotbarSlotSave{
				Type:  slot.Type,
				RefID: slot.RefID,
			}
		}
		data.Hotbar = saveHotbar
	}

	// Save Equipment
	equip, _ := ecs.GetComponent[components.EquipmentComponent](s.World, id)
	if equip != nil {
		var saveEquip [9]storage.EquipmentSlotSave
		for i, slot := range equip.Slots {
			saveEquip[i] = storage.EquipmentSlotSave{
				ItemID: slot.ItemID,
			}
		}
		data.Equipment = saveEquip
		log.Printf("PersistenceSystem: Saving %d equipment slots for %s", len(saveEquip), username)
	} else {
		log.Printf("PersistenceSystem: No EquipmentComponent found for %s", username)
	}

	// Save Spellbook
	spellbook, _ := ecs.GetComponent[components.SpellbookComponent](s.World, id)
	if spellbook != nil {
		data.UnlockedSpells = spellbook.UnlockedSpells
	} else {
		if existing.UnlockedSpells != nil {
			data.UnlockedSpells = existing.UnlockedSpells
		}
	}

	// Save UI State
	uiState, _ := ecs.GetComponent[components.UIStateComponent](s.World, id)
	if uiState != nil {
		data.OpenMenus = uiState.OpenMenus
	} else {
		data.OpenMenus = existing.OpenMenus
	}

	if err := storage.SavePlayer(data); err != nil {
		log.Printf("Failed to save player %s: %v", username, err)
		return err
	}

	log.Printf("Saved data for %s", username)
	return nil
}
