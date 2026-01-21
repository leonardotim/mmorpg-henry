package systems

import (
	"fmt"
	"henry/pkg/network"
	"henry/pkg/shared/components"
	"henry/pkg/shared/config"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type InputSystem struct {
	Client    *network.NetworkClient
	UISystem  *UISystem // Use UISystem instead of Manager
	Keys      map[string]ebiten.Key
	isRunning bool // Local toggle state
}

func NewInputSystem(client *network.NetworkClient, uiSystem *UISystem, keys map[string]ebiten.Key) *InputSystem {
	return &InputSystem{
		Client:   client,
		UISystem: uiSystem,
		Keys:     keys,
	}
}

func (s *InputSystem) SetRunning(isRunning bool) {
	s.isRunning = isRunning
}

func (s *InputSystem) Update() {
	// Movement & Actions
	input := components.InputComponent{}

	if ebiten.IsKeyPressed(s.Keys["Up"]) {
		input.Up = true
	}
	if ebiten.IsKeyPressed(s.Keys["Down"]) {
		input.Down = true
	}
	if ebiten.IsKeyPressed(s.Keys["Left"]) {
		input.Left = true
	}
	if ebiten.IsKeyPressed(s.Keys["Right"]) {
		input.Right = true
	}

	// Running Toggle (Shift)
	if inpututil.IsKeyJustPressed(s.Keys[config.ActionRun]) {
		s.isRunning = !s.isRunning
	}
	input.IsRunning = s.isRunning

	// Always capture mouse position for rotation/facing
	if !s.UISystem.IsMouseOverUI() {
		mx, my := ebiten.CursorPosition()

		// Account for camera offset
		var camX, camY float64
		state := s.Client.GetState()
		playerID := s.Client.PlayerEntityID
		for _, entity := range state.Entities {
			if entity.ID == playerID && entity.Transform != nil {
				camX = entity.Transform.X - 400 + 16
				camY = entity.Transform.Y - 300 + 16
				break
			}
		}

		input.MouseX = float64(mx) + camX
		input.MouseY = float64(my) + camY
	}

	// Active Spell
	input.ActiveSpell = s.UISystem.ActiveSpellID

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !s.UISystem.IsMouseOverUI() {
			input.Attack = true
		}
	}

	for i := 1; i <= 10; i++ {
		keyName := fmt.Sprintf("Hotbar%d", i%10)
		if inpututil.IsKeyJustPressed(s.Keys[keyName]) {
			slotIdx := i - 1
			// Check what's in this slot
			hb := s.Client.GetHotbar()
			if slotIdx < len(hb.Slots) {
				slot := hb.Slots[slotIdx]
				if slot.Type == "Spell" && slot.RefID != "" {
					// Handle Spell Locally
					def, exists := components.SpellRegistry[slot.RefID]
					if exists {
						if def.Type == "combat" {
							if s.UISystem.ActiveSpellID == slot.RefID {
								s.UISystem.ActiveSpellID = ""
								s.UISystem.AddLog("Primary attack: Weapon")
							} else {
								s.UISystem.ActiveSpellID = slot.RefID
								s.UISystem.AddLog("Primary attack: " + def.Name)
							}
							s.UISystem.SpellsWidget.ActiveSpellID = s.UISystem.ActiveSpellID
						} else {
							// Instant
							s.Client.SendCastSpell(slot.RefID)
						}
					}
				} else {
					// Item or Empty -> Send Trigger to Server
					input.HotbarTriggers[slotIdx] = true
				}
			} else {
				input.HotbarTriggers[slotIdx] = true
			}
		}
	}

	// Send Input
	s.Client.SendInput(input)
}

func (s *InputSystem) HandleGlobalKeys() {
	if inpututil.IsKeyJustPressed(s.Keys["Inventory"]) {
		s.UISystem.ToggleInventory()
	}
	if inpututil.IsKeyJustPressed(s.Keys["Equipment"]) {
		s.UISystem.ToggleEquipMenu()
	}
	if inpututil.IsKeyJustPressed(s.Keys["Spells"]) {
		s.UISystem.ToggleSpellsMenu()
	}

	if inpututil.IsKeyJustPressed(s.Keys["Bind"]) {
		s.UISystem.ToggleBindMenu()
	}

	if inpututil.IsKeyJustPressed(s.Keys["Menu"]) {
		s.UISystem.ToggleMenu()
	}

	// Debug Toggles
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		s.UISystem.ToggleDebug(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		s.UISystem.ToggleDebug(2)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		s.UISystem.ToggleDebug(3)
	}
}
