package systems

import (
	"fmt"
	"henry/pkg/network"
	"henry/pkg/shared/components"
	protocol "henry/pkg/shared/network"
	"henry/pkg/ui"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type UISystem struct {
	Client  *network.NetworkClient
	Manager *ui.Manager
	Keys    map[string]ebiten.Key

	// Windows
	LoginWindow       *ui.Window
	SignupWindow      *ui.Window
	GameMenu          *ui.Window
	Inventory         *ui.Window
	EquipWindow       *ui.Window
	SpellsWindow      *ui.Window
	KeybindingsWindow *ui.Window
	ContextMenu       *ui.ContextMenu

	// Callbacks
	OnLoginRequest func(user, pass string, signup bool)

	// Widgets
	BindWidget     *ui.InventoryWidget
	InvWidget      *ui.InventoryWidget
	SpellsWidget   *ui.SpellsWidget
	EquipWidget    *ui.EquipmentWidget
	BindWindow     *ui.Window
	KeybindButtons []struct {
		Action string
		Btn    *ui.Button
	}
	LoginInputs  []*ui.TextInput
	SignupInputs []*ui.TextInput

	// State
	selectedSlotA  int
	RebindMode     bool
	RebindAction   string
	ActiveSpellID  string
	BindingSpellID string // Spell ID waiting to be bound

	// Drag State
	DragSourceWidget ui.Element
	DragSourceIndex  int
	DragItem         string
	DragOffsetX      float64
	DragOffsetY      float64

	// Click Tracking
	pressSourceWidget ui.Element
	pressSourceIndex  int
	pressMX, pressMY  int
	wasDragging       bool

	// Debug State
	DebugFlags struct {
		ShowFPS  bool
		ShowInfo bool
		ShowLogs bool
	}
	LogHistory []string
}

func NewUISystem(client *network.NetworkClient, keys map[string]ebiten.Key) *UISystem {
	return &UISystem{
		Client:        client,
		Manager:       ui.NewManager(),
		Keys:          keys,
		selectedSlotA: -1,
	}
}

func (s *UISystem) Init() {
	// --- Bind Menu ---
	// 5x2 Grid (10 slots)
	s.BindWidget = ui.NewInventoryWidget(0, 0, 5, 2, 40)
	s.BindWidget.SlotOffset = 0
	s.BindWidget.ShowHotkeys = true
	s.BindWidget.DraggingIndex = -1

	// Height: 80 (slots) + 20 (title) = 100.
	s.BindWindow = ui.NewWindow(590, 240, 200, 100, "Binds")
	s.BindWindow.ShowScrollbar = false
	s.BindWindow.AddChild(s.BindWidget)
	s.BindWindow.Visible = false
	s.Manager.AddElement(s.BindWindow)

	// --- Equipment ---
	// Moved to Bottom Center (Left of Inv)
	// Equip was at 590, 20. Spells was at 380, 370.
	// New Equip Pos: 380, 370.
	s.EquipWidget = ui.NewEquipmentWidget(0, 0)
	s.EquipWindow = ui.NewWindow(380, 370, 200, 220, "Equipment")
	s.EquipWindow.ShowScrollbar = false
	s.EquipWindow.AddChild(s.EquipWidget)
	s.EquipWindow.Visible = false
	s.Manager.AddElement(s.EquipWindow)

	// --- Inventory ---
	// 5x5 Grid, 40px slots
	// Window Width: 5 * 40 = 200
	// Window Height: 5 * 40 + 20 (title) = 220
	// Pos: Bottom Right (800x600) -> X: 600-200=400? No, 800-200-10=590. Y: 600-220-10=370.
	s.InvWidget = ui.NewInventoryWidget(0, 0, 5, 5, 40)
	s.InvWidget.SlotOffset = 0 // Using direct 0-indexed slots matching server component
	s.Inventory = ui.NewWindow(590, 370, 200, 220, "Inventory")
	s.Inventory.ShowScrollbar = false
	s.Inventory.AddChild(s.InvWidget)
	s.Inventory.Visible = false
	s.Manager.AddElement(s.Inventory)

	// --- Spells Menu ---
	// Moved to Top Right
	// New Height: 230 to prevent scrolling
	// Pos: 590, 10 (Shifted up to make room)
	// Spells (230) + Gap (10) + Hotbar (120) + Gap (10) + Inv (230) = 600.
	s.SpellsWidget = ui.NewSpellsWidget(0, 0, 5, 5, 40) // 5x5

	// Populate Spells from Registry Order
	for i, spellID := range components.SpellList {
		if i < len(s.SpellsWidget.Slots) {
			s.SpellsWidget.Slots[i] = spellID
		}
	}

	// Sync Unlocked State from Client
	if s.Client != nil && s.Client.UnlockedSpells != nil {
		for _, spellID := range s.Client.UnlockedSpells {
			s.SpellsWidget.UnlockedSpells[spellID] = true
		}

		// Sync Cooldowns
		s.Client.Mutex.RLock()
		if s.Client.Cooldowns != nil {
			for k, v := range s.Client.Cooldowns {
				s.SpellsWidget.Cooldowns[k] = v
			}
		}
		s.Client.Mutex.RUnlock()
	} else {
		// Default unlocks for testing if empty/nil (or handle new player defaults in server)
		// For now, let's unlock "fireball" and "heal" by default if list is empty?
		// Better to do this in Server Save creation. But for now purely UI side check:
		// If using `admin`, it might have empty list.
		// Let's rely on server.
	}

	// Interaction Handler
	s.SpellsWidget.OnSpellClick = func(spellID string, isRightClick bool) {
		unlocked := s.SpellsWidget.UnlockedSpells[spellID]
		spellDef := components.SpellRegistry[spellID]

		if isRightClick {
			// Context Menu
			opts := []ui.MenuOption{}
			if unlocked {
				opts = append(opts, ui.MenuOption{Text: "Cast", Action: func() {
					// Replicate Primary Action Logic
					if spellDef.Type == "combat" {
						if s.ActiveSpellID == spellID {
							s.ActiveSpellID = ""
							s.AddLog("Primary attack: Weapon")
						} else {
							s.ActiveSpellID = spellID
							s.AddLog("Primary attack: " + spellDef.Name)
						}
						s.SpellsWidget.ActiveSpellID = s.ActiveSpellID
					} else {
						s.AddLog("Casting " + spellDef.Name)
						s.Client.SendCastSpell(spellID)
					}
				}})
				opts = append(opts, ui.MenuOption{Text: "Bind", Action: func() {
					// Auto-Bind to First Free Slot
					hb := s.Client.GetHotbar()
					freeSlot := -1
					for i, slot := range hb.Slots {
						if slot.RefID == "" {
							freeSlot = i
							break
						}
					}

					if freeSlot != -1 {
						s.SendHotbarAction("Bind", freeSlot, "Spell", spellID, -1)
						s.AddLog(fmt.Sprintf("Bound %s to Slot %d", spellDef.Name, freeSlot+1))
						s.BindWindow.Visible = true
					} else {
						s.AddLog("No free hotbar slots!")
					}
				}})
			} else {
				opts = append(opts, ui.MenuOption{Text: "Locked", Action: nil})
			}
			mx, my := ebiten.CursorPosition()
			minX := s.SpellsWindow.X
			minY := s.SpellsWindow.Y
			maxX := minX + s.SpellsWindow.Width
			maxY := minY + s.SpellsWindow.Height
			s.ContextMenu.Show(float64(mx), float64(my), opts, minX, minY, maxX, maxY)
		} else {
			// Left Click
			if unlocked {
				// Combat vs Instant
				if spellDef.Type == "combat" {
					// Toggle Active Spell
					if s.ActiveSpellID == spellID {
						s.ActiveSpellID = ""
						s.AddLog("Primary attack: Weapon")
					} else {
						s.ActiveSpellID = spellID
						s.AddLog("Primary attack: " + spellDef.Name)
					}
					// Update Widget visual
					s.SpellsWidget.ActiveSpellID = s.ActiveSpellID
				} else {
					// Instant actions (Heal, Teleport)
					s.AddLog("Casting " + spellDef.Name)
					s.Client.SendCastSpell(spellID)
				}
			} else {
				s.AddLog(spellDef.Name + " is locked.")
			}
		}
	}

	// --- Spells Menu ---
	// Moved to Top Right
	// Height: 220 (Fits exactly 5 rows + title)
	// Pos: 590, 30.
	// Spells (220) + Gap 10 => Ends 260.
	// REUSED INSTANCE FROM TOP of Init()

	s.SpellsWindow = ui.NewWindow(590, 30, 200, 220, "Spells")
	s.SpellsWindow.ShowScrollbar = false
	s.SpellsWindow.AddChild(s.SpellsWidget)
	s.SpellsWindow.Visible = false
	s.Manager.AddElement(s.SpellsWindow)

	// Update BindWindow Y
	// Spells ends 250 (30+220). Gap 10 => 260.
	s.BindWindow.Y = 260
	// Inventory is at 370.
	// If Binds starts 270. Height 100. Ends 370. Touching Inventory.
	// We need 10px gap.
	// If Spells Y=30. Height 230. Bottom=260.
	// Gap 10px. Binds Y=270.
	// Binds Height 100. Bottom=370.
	// Gap 10px. Inventory Y must be 380?
	// Inventory is 370.
	// So we are squeezed.
	// If I squeeze gaps to 5px?
	// Spells Y=30. H=230. Bot=260.
	// Gap 5px. Binds Y=265.
	// Binds H=100. Bot=365.
	// Gap 5px. Inventory Y=370.
	// This fits!

	// Populate Spells from Registry Order
	for i, spellID := range components.SpellList {
		if i < len(s.SpellsWidget.Slots) {
			s.SpellsWidget.Slots[i] = spellID
		}
	}

	// Sync Unlocked State from Client
	if s.Client != nil && s.Client.UnlockedSpells != nil {
		for _, spellID := range s.Client.UnlockedSpells {
			s.SpellsWidget.UnlockedSpells[spellID] = true
		}

		// Sync Cooldowns
		s.Client.Mutex.RLock()
		if s.Client.Cooldowns != nil {
			for k, v := range s.Client.Cooldowns {
				s.SpellsWidget.Cooldowns[k] = v
			}
		}
		s.Client.Mutex.RUnlock()
	} else {
		// Default unlocks for testing if empty/nil (or handle new player defaults in server)
		// For now, let's unlock "fireball" and "heal" by default if list is empty?
		// Better to do this in Server Save creation. But for now purely UI side check:
		// If using `admin`, it might have empty list.
		// Let's rely on server.
	}

	// Context Menu
	s.ContextMenu = ui.NewContextMenu()
	s.Manager.AddElement(s.ContextMenu)

	// Inventory Drag-Drop Logic
	s.InvWidget.OnItemDrop = func(fromIndex, toIndex int) {
		if fromIndex == toIndex {
			return
		}
		s.SendInventoryAction("Swap", fromIndex, toIndex)
	}

	// Double right-click handling removed. Logic consolidated in Update().

	// --- Login/Signup Windows ---
	s.InitAuthUI()

	// --- Keybindings Window ---
	s.InitKeybindingsUI()

	// --- Game Menu ---
	s.GameMenu = ui.NewWindow(300, 200, 200, 200, "Menu")

	resumeBtn := ui.NewButton(10, 30, 180, 30, "Resume", func() {
		s.GameMenu.Visible = false
	})
	s.GameMenu.AddChild(resumeBtn)

	kbBtn := ui.NewButton(10, 70, 180, 30, "Keybindings", func() {
		s.GameMenu.Visible = false
		s.KeybindingsWindow.Visible = true
		s.RefreshKeybinds()
	})
	s.GameMenu.AddChild(kbBtn)

	s.GameMenu.Visible = false
	s.Manager.AddElement(s.GameMenu)

	s.AddLog("Welcome to Henry!")
}

func (s *UISystem) InitKeybindingsUI() {
	kbWidth := 300.0
	kbHeight := 300.0
	kbMenu := ui.NewWindow(
		(800-kbWidth)/2,
		(600-kbHeight)/2,
		kbWidth, kbHeight,
		"Keybindings",
	)

	actions := []string{"Menu", "Up", "Down", "Left", "Right", "Run", "Inventory", "Equipment", "Spells", "Bind",
		"Hotbar1", "Hotbar2", "Hotbar3", "Hotbar4", "Hotbar5", "Hotbar6", "Hotbar7", "Hotbar8", "Hotbar9", "Hotbar0"}
	yOffset := 30.0

	for _, action := range actions {
		act := action
		lbl := ui.NewLabel(20, yOffset+5, act+":")
		kbMenu.AddChild(lbl)

		var onClick func()
		if act == "Menu" {
			// Lock Menu to Escape
			onClick = func() {} // Do nothing
		} else {
			onClick = func() {
				s.RebindAction = act
				s.RebindMode = true
				s.GameMenu.Visible = false // Ensure menu logic doesn't interfere?
				// Actually rebind mode blocks other inputs.
			}
		}

		btn := ui.NewButton(120, yOffset, 100, 25, s.GetKeyName(act), onClick)
		if act == "Menu" {
			// Maybe style it differently?
			btn.Style = ui.ButtonStyleSecondary // Visual indication it's different
		}

		kbMenu.AddChildOption(btn, false)

		s.KeybindButtons = append(s.KeybindButtons, struct {
			Action string
			Btn    *ui.Button
		}{act, btn})

		yOffset += 30.0
	}

	kbMenu.SetBackButton(func() {
		kbMenu.Visible = false
		s.GameMenu.Visible = true
	})

	kbMenu.Visible = false
	s.KeybindingsWindow = kbMenu
	s.Manager.AddElement(kbMenu)
}

func (s *UISystem) GetKeyName(action string) string {
	if k, ok := s.Keys[action]; ok {
		return k.String()
	}
	return "?"
}

func (s *UISystem) RefreshKeybinds() {
	for _, kb := range s.KeybindButtons {
		kb.Btn.Text = s.GetKeyName(kb.Action)
	}
}

func (s *UISystem) InitAuthUI() {
	loginW := 300.0
	loginH := 280.0 // Increased height for better spacing
	x := (800.0 - loginW) / 2
	y := (600.0 - loginH) / 2

	// --- Login Window ---
	loginWin := ui.NewWindow(x, y, loginW, loginH, "Login")
	loginWin.Visible = true

	lblUser := ui.NewLabel(20, 30, "Username:")
	loginWin.AddChild(lblUser)

	inputUser := ui.NewTextInput(20, 50, 260, 30, "Username")
	loginWin.AddChild(inputUser)

	lblPass := ui.NewLabel(20, 90, "Password:")
	loginWin.AddChild(lblPass)

	inputPass := ui.NewTextInput(20, 110, 260, 30, "Password")
	inputPass.IsPassword = true
	loginWin.AddChild(inputPass)

	s.LoginInputs = []*ui.TextInput{inputUser, inputPass}

	// Login Action (Primary)
	btnLogin := ui.NewButton(20, 160, 260, 40, "Login", func() {
		if s.OnLoginRequest != nil {
			go s.OnLoginRequest(inputUser.Text, inputPass.Text, false)
		}
	})
	loginWin.AddChild(btnLogin)

	// Switch to Signup (Secondary)
	// Moved down slightly to 220
	btnToSignup := ui.NewSecondaryButton(20, 220, 260, 30, "Create Account", func() {
		s.LoginWindow.Visible = false
		s.SignupWindow.Visible = true
		// Clear inputs?
		inputUser.Text = ""
		inputPass.Text = ""
	})
	loginWin.AddChild(btnToSignup)

	s.LoginWindow = loginWin
	s.Manager.AddElement(loginWin)

	// --- Signup Window ---
	signupWin := ui.NewWindow(x, y, loginW, loginH, "Create Account")
	signupWin.Visible = false

	lblUserS := ui.NewLabel(20, 30, "Username:")
	signupWin.AddChild(lblUserS)

	inputUserS := ui.NewTextInput(20, 50, 260, 30, "Username")
	signupWin.AddChild(inputUserS)

	lblPassS := ui.NewLabel(20, 90, "Password:")
	signupWin.AddChild(lblPassS)

	inputPassS := ui.NewTextInput(20, 110, 260, 30, "Password")
	inputPassS.IsPassword = true
	signupWin.AddChild(inputPassS)

	s.SignupInputs = []*ui.TextInput{inputUserS, inputPassS}

	// Signup Action (Primary)
	btnSignup := ui.NewButton(20, 160, 260, 40, "Sign Up", func() {
		if s.OnLoginRequest != nil {
			go s.OnLoginRequest(inputUserS.Text, inputPassS.Text, true)
		}
	})
	signupWin.AddChild(btnSignup)

	// Switch Back to Login (Secondary)
	btnBack := ui.NewSecondaryButton(20, 220, 260, 30, "Back to Login", func() {
		s.SignupWindow.Visible = false
		s.LoginWindow.Visible = true
		inputUserS.Text = ""
		inputPassS.Text = ""
	})
	signupWin.AddChild(btnBack)

	s.SignupWindow = signupWin
	s.Manager.AddElement(signupWin)
}

func (s *UISystem) RegisterDisconnectCallback(onDisconnect func()) {
	quitBtn := ui.NewButton(10, 110, 180, 30, "Disconnect", func() {
		if onDisconnect != nil {
			onDisconnect()
		}
	})
	s.GameMenu.AddChild(quitBtn)
}

func (s *UISystem) ResetUI() {
	if s.Inventory != nil {
		s.Inventory.Visible = false
	}
	if s.SpellsWindow != nil {
		s.SpellsWindow.Visible = false
	}
	if s.EquipWindow != nil {
		s.EquipWindow.Visible = false
	}
	if s.BindWindow != nil {
		s.BindWindow.Visible = false
	}
	if s.GameMenu != nil {
		s.GameMenu.Visible = false
	}
	if s.KeybindingsWindow != nil {
		s.KeybindingsWindow.Visible = false
	}
	if s.ContextMenu != nil {
		s.ContextMenu.Visible = false
	}
	if s.LoginWindow != nil {
		s.LoginWindow.Visible = true
	}
}

func (s *UISystem) RegisterLoginCallback(cb func(user, pass string, isSignup bool)) {
	s.OnLoginRequest = cb
}

func (s *UISystem) HideLogin() {
	if s.LoginWindow != nil {
		s.LoginWindow.Visible = false
	}
	if s.SignupWindow != nil {
		s.SignupWindow.Visible = false
	}
	// BindWindow visibility is handled by ApplyOpenMenus
}

func (s *UISystem) Update() {
	s.Manager.Update()

	// Determine Active Inputs
	var activeInputs []*ui.TextInput
	var isSignup bool

	if s.LoginWindow != nil && s.LoginWindow.Visible {
		activeInputs = s.LoginInputs
		isSignup = false
	} else if s.SignupWindow != nil && s.SignupWindow.Visible {
		activeInputs = s.SignupInputs
		isSignup = true
	}

	// Handle Tab Navigation and Enter
	if activeInputs != nil {
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			// Find currently focused
			current := -1
			for i, input := range activeInputs {
				if input.Focused {
					current = i
					break
				}
			}

			// Determine direction
			delta := 1
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				delta = -1
			}

			// Calculate next
			next := 0
			if current != -1 {
				next = (current + delta) % len(activeInputs)
				if next < 0 {
					next = len(activeInputs) - 1
				}
			}

			// Apply Focus
			for i, input := range activeInputs {
				input.Focused = (i == next)
			}
		}

		// Handle Enter Key (Trigger Action)
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
			if len(activeInputs) >= 2 {
				user := activeInputs[0].Text
				pass := activeInputs[1].Text
				if s.OnLoginRequest != nil {
					go s.OnLoginRequest(user, pass, isSignup)
				}
			}
		}
	} else {
		// Clear focus if not visible
		for _, input := range s.LoginInputs {
			input.Focused = false
		}
		if s.SignupInputs != nil {
			for _, input := range s.SignupInputs {
				input.Focused = false
			}
		}
	}

	if s.RebindMode {
		// Find pressed key
		for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
			if inpututil.IsKeyJustPressed(k) {
				// Prevent mapping Escape to anything
				if k == ebiten.KeyEscape {
					// Cancel rebind
					s.RebindMode = false
					s.RebindAction = ""
					s.RefreshKeybinds()
					return
				}

				// Avoid rebinding Escape/Menu if essential? Allow everything for now.
				s.Keys[s.RebindAction] = k
				s.RebindMode = false
				s.RebindAction = ""
				s.RefreshKeybinds()

				// Send Update to Server
				if s.Client != nil {
					// Convert ebiten.Key (int) to generic int map for protocol
					bindings := make(map[string]int)
					for action, key := range s.Keys {
						bindings[action] = int(key)
					}

					packet := protocol.Packet{
						Type: protocol.PacketUpdateKeybindings,
						Data: protocol.UpdateKeybindingsPacket{
							Keybindings: bindings,
						},
					}
					if s.Client.Encoder != nil {
						s.Client.Encoder.Encode(packet)
					}
				}

				return // Found one, exit
			}
		}

		// Allow canceling with mouse click outside? May vary.
		return // If rebind mode, skip other updates like inventory sync?
	}

	// Sync Data
	inv := s.Client.GetInventory()
	if inv.Capacity > 0 {
		// Sync Inventory Widget
		for i := range s.InvWidget.Slots {
			if i < len(inv.Slots) {
				s.InvWidget.Slots[i] = inv.Slots[i].ItemID // Assuming Protocol structure
				// Need mapping Index -> ItemID. inv.Slots is a slice of structs?
				// inv.Slots is []struct{Index, ItemID, Quantity}
			} else {
				// s.InvWidget.Slots[i] = "" // No, this logic is flawed if sparse
			}
		}
		// Clear first
		for i := range s.InvWidget.Slots {
			s.InvWidget.Slots[i] = ""
		}
		for _, v := range inv.Slots {
			if v.Index >= 0 && v.Index < len(s.InvWidget.Slots) {
				s.InvWidget.Slots[v.Index] = v.ItemID
			}
		}
	}

	// Sync Hotbar
	hb := s.Client.GetHotbar()
	// Check for changes (simple check or always copy?)
	// Copy always for now, it's cheap.
	for i := range s.BindWidget.Slots {
		if i < len(hb.Slots) {
			newVal := hb.Slots[i].RefID
			if s.BindWidget.Slots[i] != newVal {
				s.AddLog(fmt.Sprintf("Hotbar update: Slot %d -> %s", i+1, newVal))
				s.BindWidget.Slots[i] = newVal
			}
		} else {
			s.BindWidget.Slots[i] = ""
		}
	}

	eq := s.Client.GetEquipment()
	// Sync Equip Widget
	for i := range s.EquipWidget.Slots {
		if i < len(eq.Slots) {
			s.EquipWidget.Slots[i] = eq.Slots[i].ItemID
		}
	}

	// --- Global Drag & Click Logic ---
	mx, my := ebiten.CursorPosition()

	// 1. Handle Press (Start Tracking)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s.pressSourceWidget = nil
		s.pressSourceIndex = -1
		s.pressMX, s.pressMY = mx, my
		s.wasDragging = false

		// Identify what was pressed
		if s.BindWindow.Visible && s.BindWidget.IsHovered(mx, my) {
			s.pressSourceWidget = s.BindWidget
			s.pressSourceIndex = s.BindWidget.GetSlotAt(mx, my)
		} else if s.Inventory.Visible && s.InvWidget.IsHovered(mx, my) {
			s.pressSourceWidget = s.InvWidget
			s.pressSourceIndex = s.InvWidget.GetSlotAt(mx, my)
		} else if s.EquipWindow.Visible && s.EquipWidget.IsHovered(mx, my) {
			s.pressSourceWidget = s.EquipWidget
			s.pressSourceIndex = s.EquipWidget.GetSlotAt(mx, my)
		}
	}

	// 2. Handle Drag Start (If moved enough while held)
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && s.pressSourceWidget != nil && s.DragSourceWidget == nil {
		dx := mx - s.pressMX
		dy := my - s.pressMY
		if (dx*dx + dy*dy) > 25 { // 5px threshold
			idx := s.pressSourceIndex
			w := s.pressSourceWidget

			// Only start drag if slot is not empty
			var item string
			if iw, ok := w.(*ui.InventoryWidget); ok {
				item = iw.Slots[idx]
			} else if ew, ok := w.(*ui.EquipmentWidget); ok {
				item = ew.Slots[idx]
			}

			if item != "" {
				s.DragSourceWidget = w
				s.DragSourceIndex = idx
				s.DragItem = item
				s.wasDragging = true

				if iw, ok := w.(*ui.InventoryWidget); ok {
					iw.HiddenIndex = idx
				} else if ew, ok := w.(*ui.EquipmentWidget); ok {
					ew.HiddenIndex = idx
				}
			}
		}
	}

	// 3. Handle Release
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if s.DragSourceWidget != nil {
			// DROP logic
			targetWidget := (*ui.InventoryWidget)(nil)
			targetIndex := -1

			if s.BindWindow.Visible && s.BindWidget.IsHovered(mx, my) {
				targetWidget = s.BindWidget
				targetIndex = s.BindWidget.GetSlotAt(mx, my)
			} else if s.Inventory.Visible && s.InvWidget.IsHovered(mx, my) {
				targetWidget = s.InvWidget
				targetIndex = s.InvWidget.GetSlotAt(mx, my)
			} else if s.EquipWindow.Visible && s.EquipWidget.IsHovered(mx, my) {
				targetIndex = s.EquipWidget.GetSlotAt(mx, my)
				s.HandleDropToEquip(s.DragSourceWidget, s.DragSourceIndex, targetIndex)
				goto EndDrag
			}

			if targetWidget != nil && targetIndex != -1 {
				s.HandleDrop(s.DragSourceWidget, s.DragSourceIndex, targetWidget, targetIndex)
			}

		EndDrag:
			// Reset Hidden State
			if iw, ok := s.DragSourceWidget.(*ui.InventoryWidget); ok {
				iw.HiddenIndex = -1
			} else if ew, ok := s.DragSourceWidget.(*ui.EquipmentWidget); ok {
				ew.HiddenIndex = -1
			}
			s.DragSourceWidget = nil
			s.DragSourceIndex = -1
			s.DragItem = ""
		} else if s.pressSourceWidget != nil && !s.wasDragging {
			// CLICK logic -> Perform Primary action
			idx := s.pressSourceIndex
			if idx != -1 {
				if s.pressSourceWidget == s.BindWidget {
					// Hotbar Click
					if s.BindingSpellID != "" {
						// Perform Bind
						s.SendHotbarAction("Bind", idx, "Spell", s.BindingSpellID, -1)
						s.AddLog(fmt.Sprintf("Bound spell to slot %d", idx+1))
						s.BindingSpellID = ""
					} else {
						// Normal Hotbar interaction (Use/Select)
						// Maybe toggle active spell active?
						// Server handles "HotbarTriggers" from keys.
						// Mouse click could simulate key press?
						// For now, allow binding as primary interaction mode if requested.
					}
				} else if s.pressSourceWidget == s.InvWidget {
					if s.InvWidget.Slots[idx] != "" {
						s.SendInventoryAction("Primary", idx, -1)
					}
				} else if s.pressSourceWidget == s.EquipWidget {
					if s.EquipWidget.Slots[idx] != "" {
						s.SendEquipmentAction("Unequip", idx, -1)
					}
				}
			}
		}

		s.pressSourceWidget = nil
		s.pressSourceIndex = -1
	}

	// 4. Right Click Handling (Always show menu)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if s.Inventory.Visible && s.InvWidget.IsHovered(mx, my) {
			idx := s.InvWidget.GetSlotAt(mx, my)
			if idx != -1 && s.InvWidget.Slots[idx] != "" {
				s.OpenContextMenu(s.InvWidget, idx, mx, my)
				return
			}
		}
		if s.BindWindow.Visible && s.BindWidget.IsHovered(mx, my) {
			idx := s.BindWidget.GetSlotAt(mx, my)
			if idx != -1 {
				s.OpenContextMenu(s.BindWidget, idx, mx, my)
				return
			}
		}
		if s.EquipWindow.Visible && s.EquipWidget.IsHovered(mx, my) {
			idx := s.EquipWidget.GetSlotAt(mx, my)
			if idx != -1 && s.EquipWidget.Slots[idx] != "" {
				s.OpenEquipContextMenu(idx, mx, my)
				return
			}
		}
	}
}

func (s *UISystem) Draw(screen *ebiten.Image) {
	s.Manager.Draw(screen)

	// Draw Dragged Item
	if s.DragSourceWidget != nil && s.DragItem != "" {
		mx, my := ebiten.CursorPosition()
		ebitenutil.DebugPrintAt(screen, s.DragItem[:1], mx, my)
		// Or draw a box
	}

	// Draw Spell Tooltips (Topmost)
	if s.SpellsWindow.Visible && s.SpellsWidget.HoveredSpellID != "" {
		sw := s.SpellsWidget
		spellID := sw.HoveredSpellID
		spellDef := components.SpellRegistry[spellID]
		unlocked := sw.UnlockedSpells[spellID]

		msg := spellDef.Name
		if !unlocked {
			msg += " (LOCKED)"
		}

		// Style
		tipWidth := float64(len(msg)*7 + 10)
		tipHeight := 20.0

		drawX := sw.TooltipX
		drawY := sw.TooltipY

		// Screen Bounds Check
		// User requested: "tooltip should only appear within the bounds of the spells menu"
		// We clamp to Window Bounds instead of Screen bounds.

		winX := s.SpellsWindow.X
		winW := s.SpellsWindow.Width

		if drawX+tipWidth > winX+winW {
			drawX = winX + winW - tipWidth - 5
		}
		// Also check Left bound?
		if drawX < winX {
			drawX = winX + 5
		}

		// Background
		ebitenutil.DrawRect(screen, drawX, drawY, tipWidth, tipHeight, color.RGBA{0, 0, 0, 220})

		ebitenutil.DebugPrintAt(screen, msg, int(drawX+5), int(drawY+2))
	}

	s.DrawDebug(screen)
}

func (s *UISystem) ToggleDebug(mode int) {
	switch mode {
	case 1:
		s.DebugFlags.ShowFPS = !s.DebugFlags.ShowFPS
	case 2:
		s.DebugFlags.ShowInfo = !s.DebugFlags.ShowInfo
	case 3:
		s.DebugFlags.ShowLogs = !s.DebugFlags.ShowLogs
	}

	// Sync with server
	if s.Client != nil {
		settings := map[string]bool{
			"ShowFPS":  s.DebugFlags.ShowFPS,
			"ShowInfo": s.DebugFlags.ShowInfo,
			"ShowLogs": s.DebugFlags.ShowLogs,
		}
		s.Client.SendUpdateDebugSettings(settings)
	}
}

func (s *UISystem) AddLog(msg string) {
	s.LogHistory = append(s.LogHistory, msg)
	if len(s.LogHistory) > 10 {
		s.LogHistory = s.LogHistory[len(s.LogHistory)-10:]
	}
}

func (s *UISystem) DrawDebug(screen *ebiten.Image) {
	// F1: FPS (Top Left)
	if s.DebugFlags.ShowFPS {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS: %0.2f\nTPS: %0.2f", ebiten.ActualFPS(), ebiten.ActualTPS()), 5, 5)
	}

	// F2: Info (Top Right)
	if s.DebugFlags.ShowInfo {
		mx, my := ebiten.CursorPosition()
		msg := fmt.Sprintf("Mouse: %d, %d", mx, my)
		// Calculate X based on screen width (800) and text length approx
		x := 800 - 120
		ebitenutil.DebugPrintAt(screen, msg, x+5, 5)
	}

	// F3: Logs (Bottom Left)
	if s.DebugFlags.ShowLogs {
		logH := len(s.LogHistory) * 15
		logY := 600 - logH - 5

		for _, log := range s.LogHistory {
			ebitenutil.DebugPrintAt(screen, log, 5, logY)
			logY += 15
		}
	}
}

// Helpers for InputSystem
func (s *UISystem) ToggleInventory() {
	s.Inventory.Visible = !s.Inventory.Visible
	// Optionally toggle BindMenu with Inventory? Or separate?
	// User requested separate hotkey B.
	// But "drawn directly above inventory" might imply they act together.
	// Let's allow independent toggle but maybe default bind B opens just bind menu.
	s.SyncUIState()
}

func (s *UISystem) ToggleBindMenu() {
	s.BindWindow.Visible = !s.BindWindow.Visible
	s.SyncUIState()
}

func (s *UISystem) ToggleMenu() {
	if s.RebindMode {
		return // Ignore menu toggle during rebind
	}
	if s.KeybindingsWindow != nil && s.KeybindingsWindow.Visible {
		s.KeybindingsWindow.Visible = false
		s.GameMenu.Visible = true
		return
	}
	s.GameMenu.Visible = !s.GameMenu.Visible
}

func (s *UISystem) IsMenuVisible() bool {
	return s.GameMenu.Visible
}

func (s *UISystem) IsInputCaptured() bool {
	return s.RebindMode || s.GameMenu.Visible ||
		(s.KeybindingsWindow != nil && s.KeybindingsWindow.Visible) ||
		(s.LoginWindow != nil && s.LoginWindow.Visible) ||
		(s.SignupWindow != nil && s.SignupWindow.Visible)
}

func (s *UISystem) IsMouseOverUI() bool {
	return s.Manager.IsMouseOverUI()
}

func (s *UISystem) SendInventoryAction(actionType string, slotA, slotB int) {
	action := protocol.Packet{
		Type: protocol.PacketInventoryAction,
		Data: protocol.InventoryActionPacket{
			ActionType: actionType,
			SlotA:      slotA,
			SlotB:      slotB,
		},
	}
	if s.Client.Encoder != nil {
		s.Client.Encoder.Encode(action)
	}
}

func (s *UISystem) SendHotbarAction(actionType string, slotIndex int, targetType, targetRef string, slotIndexB int) {
	action := protocol.Packet{
		Type: protocol.PacketHotbarAction,
		Data: protocol.HotbarActionPacket{
			ActionType:  actionType,
			SlotIndex:   slotIndex,
			TargetType:  targetType,
			TargetRefID: targetRef,
			SlotIndexB:  slotIndexB,
		},
	}
	if s.Client.Encoder != nil {
		s.Client.Encoder.Encode(action)
	}
}

func (s *UISystem) ToggleEquipMenu() {
	s.EquipWindow.Visible = !s.EquipWindow.Visible
	s.SyncUIState()
}

func (s *UISystem) ToggleSpellsMenu() {
	s.SpellsWindow.Visible = !s.SpellsWindow.Visible
	s.SyncUIState()
}

func (s *UISystem) SendEquipmentAction(actionName string, slot int, invSlot int) {
	action := protocol.Packet{
		Type: protocol.PacketEquipmentAction,
		Data: protocol.EquipmentActionPacket{
			Action:  actionName,
			Slot:    slot,
			InvSlot: invSlot,
		},
	}
	if s.Client.Encoder != nil {
		s.Client.Encoder.Encode(action)
	}
}

func (s *UISystem) HandleDrop(srcW ui.Element, srcIdx int, destW ui.Element, destIdx int) {
	// Source: Inventory
	if srcW == s.InvWidget {
		// Dest: Inventory -> Swap
		if destW == s.InvWidget {
			if srcIdx != destIdx {
				s.SendInventoryAction("Swap", srcIdx, destIdx)
			}
		} else if destW == s.BindWidget {
			// Dest: Bind -> Bind Item (Create Reference)
			itemID := s.InvWidget.Slots[srcIdx]
			if itemID != "" {
				s.SendHotbarAction("Bind", destIdx, "Item", itemID, -1)
			}
		}
	} else if srcW == s.BindWidget {
		// Source: Bind
		if destW == s.BindWidget {
			// Dest: Bind -> Swap Binds
			if srcIdx != destIdx {
				s.SendHotbarAction("Swap", srcIdx, "", "", destIdx)
			}
		}
	}
}

func (s *UISystem) HandleDropToEquip(srcW ui.Element, srcIdx int, destSlot int) {
	if srcW == s.InvWidget {
		s.SendEquipmentAction("Equip", destSlot, srcIdx)
	} else if srcW == s.EquipWidget {
		// Drag from one equip slot to another? Probably not allowed unless same type.
		// For now, simplify and only allow Inv -> Equip.
	}
}

func (s *UISystem) OpenEquipContextMenu(slotIndex int, mx, my int) {
	itemID := s.EquipWidget.Slots[slotIndex]
	if itemID == "" {
		return
	}

	actions := []ui.MenuOption{
		{
			Text: "Unequip",
			Action: func() {
				s.SendEquipmentAction("Unequip", slotIndex, -1)
			},
		},
	}
	minX := s.EquipWindow.X
	minY := s.EquipWindow.Y
	maxX := minX + s.EquipWindow.Width
	maxY := minY + s.EquipWindow.Height
	s.ContextMenu.Show(float64(mx), float64(my), actions, minX, minY, maxX, maxY)
}

func (s *UISystem) OpenContextMenu(w ui.Element, index int, mx, my int) {
	// Check if this is InvWidget
	iw, ok := w.(*ui.InventoryWidget)
	if !ok {
		return
	}
	itemID := iw.Slots[index]
	if itemID == "" {
		return
	}

	primaryText := "Use"
	if strings.Contains(itemID, "potion") {
		primaryText = "Drink"
	} else if strings.Contains(itemID, "sword") || strings.Contains(itemID, "bow") {
		primaryText = "Equip"
	}

	var actions []ui.MenuOption
	if w == s.BindWidget {
		actions = []ui.MenuOption{
			{
				Text: "Unbind",
				Action: func() {
					s.SendHotbarAction("Bind", index, "", "", -1) // Clear Ref
				},
			},
		}
	} else {
		// Inventory Widget
		actions = []ui.MenuOption{
			{
				Text: primaryText,
				Action: func() {
					if primaryText == "Equip" {
						// Need to find which slot it goes into.
						// HACK: Server handles validation, but client needs to pick a slot.
						// Or we send "Equip" with -1 and server picks?
						// Let's send a special action for "Auto-Equip" if we want.
						// For now, dragging is better.
						s.SendInventoryAction("Primary", index, -1)
					} else {
						s.SendInventoryAction("Primary", index, -1)
					}
				},
			},
			{
				Text: "Bind",
				Action: func() {
					// Find empty bind slot
					targetSlot := -1
					for i, ref := range s.BindWidget.Slots {
						if ref == "" {
							targetSlot = i
							break
						}
					}
					if targetSlot == -1 {
						targetSlot = 0 // Overwrite first slot if full
					}

					s.SendHotbarAction("Bind", targetSlot, "Item", itemID, -1)
				},
			},
			{
				Text: "Drop",
				Action: func() {
					s.SendInventoryAction("Drop", index, -1)
				},
			},
		}
	}

	var minX, minY, maxX, maxY float64
	if w == s.BindWidget {
		minX = s.BindWindow.X
		minY = s.BindWindow.Y
		maxX = minX + s.BindWindow.Width
		maxY = minY + s.BindWindow.Height
	} else if w == s.InvWidget {
		minX = s.Inventory.X
		minY = s.Inventory.Y
		maxX = minX + s.Inventory.Width
		maxY = minY + s.Inventory.Height
	} else {
		// Fallback
		minX, minY = 0, 0
		maxX, maxY = 800, 600
	}

	s.ContextMenu.Show(float64(mx), float64(my), actions, minX, minY, maxX, maxY)
}
func (s *UISystem) ApplyOpenMenus(openMenus map[string]bool) {
	if openMenus == nil {
		// Default State if nothing saved (New Player or first time with feature)
		// Binds: Shown
		// Spells, Inv, Equip: Hidden
		if s.BindWindow != nil {
			s.BindWindow.Visible = true
		}
		return
	}

	if s.Inventory != nil {
		s.Inventory.Visible = openMenus["Inventory"]
	}
	if s.SpellsWindow != nil {
		s.SpellsWindow.Visible = openMenus["Spells"]
	}
	if s.EquipWindow != nil {
		s.EquipWindow.Visible = openMenus["Equipment"]
	}
	if s.BindWindow != nil {
		s.BindWindow.Visible = openMenus["Binds"]
	}
	// Character?
}

func (s *UISystem) SyncUIState() {
	if s.Client == nil {
		return
	}

	openMenus := make(map[string]bool)
	if s.Inventory != nil && s.Inventory.Visible {
		openMenus["Inventory"] = true
	}
	if s.SpellsWindow != nil && s.SpellsWindow.Visible {
		openMenus["Spells"] = true
	}
	if s.EquipWindow != nil && s.EquipWindow.Visible {
		openMenus["Equipment"] = true
	}
	if s.BindWindow != nil && s.BindWindow.Visible {
		openMenus["Binds"] = true
	}

	packet := protocol.Packet{
		Type: protocol.PacketUpdateUIState,
		Data: protocol.UpdateUIStatePacket{OpenMenus: openMenus},
	}

	if s.Client.Encoder != nil {
		s.Client.Encoder.Encode(packet)
	}
}
