package server

import (
	"encoding/gob"
	"image/color"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"henry/pkg/characters"
	"henry/pkg/items"
	"henry/pkg/network"
	"henry/pkg/server/systems"
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
	protocol "henry/pkg/shared/network"
	"henry/pkg/shared/world"
	"henry/pkg/storage"
)

type Player struct {
	Conn      net.Conn
	Encoder   *gob.Encoder
	Decoder   *gob.Decoder
	EntityID  ecs.Entity
	Username  string
	PrevInput components.InputComponent
}

type GameServer struct {
	World             *ecs.World
	Players           map[ecs.Entity]*Player
	Mutex             sync.RWMutex
	MovementSystem    *systems.MovementSystem
	NetworkSystem     *systems.NetworkSystem
	PersistenceSystem *systems.PersistenceSystem
	AISystem          *systems.AISystem
	Maps              map[int]*world.Map // Support multiple levels
}

func NewGameServer() *GameServer {
	worldECS := ecs.NewWorld()

	// Load Maps
	maps := make(map[int]*world.Map)
	m0, err := world.LoadMap("data/maps/level_0.json")
	if err != nil {
		panic(err) // panic on startup if map missing
	}
	maps[0] = m0

	// Initialize Server
	gs := &GameServer{
		World:   worldECS,
		Players: make(map[ecs.Entity]*Player),
		Maps:    maps,
	}

	gs.MovementSystem = systems.NewMovementSystem(worldECS, maps)
	gs.NetworkSystem = systems.NewNetworkSystem(worldECS)
	gs.PersistenceSystem = systems.NewPersistenceSystem(worldECS)
	gs.AISystem = systems.NewAISystem(worldECS, maps)

	return gs
}

func (s *GameServer) Run(port string) {
	protocol.RegisterGobTypes()
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", port, err)
	}
	log.Printf("Server listening on %s", port)

	// Start WebSocket Server
	go func() {
		log.Printf("WebSocket Server listening on :8081/ws")
		network.StartWebSocketServer(":8081", s.HandleConnection)
	}()

	// Spawn Entities from Maps
	for _, m := range s.Maps {
		for _, spawner := range m.Spawners {
			s.SpawnCharacter(spawner.X, spawner.Y, spawner.CharacterID)
		}
	}

	// Game Loop
	go s.GameLoop()

	// Graceful Shutdown Handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		s.Mutex.Lock()
		for id, player := range s.Players {
			log.Printf("Saving player %s on shutdown...", player.Username)
			s.PersistenceSystem.SavePlayer(id, player.Username)
		}
		s.Mutex.Unlock()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go s.HandleConnection(conn)
	}
}

func (s *GameServer) SpawnCharacter(x, y float64, charID string) {
	def, exists := characters.Get(charID)
	if !exists {
		return
	}

	npc := s.World.NewEntity()
	s.World.AddComponent(npc, components.TransformComponent{X: x, Y: y})
	s.World.AddComponent(npc, components.PhysicsComponent{Speed: def.Speed})
	s.World.AddComponent(npc, components.SpriteComponent{Width: def.SpriteWidth, Height: def.SpriteHeight, Color: def.Color})
	s.World.AddComponent(npc, components.StatsComponent{MaxHealth: def.MaxHealth, CurrentHealth: def.MaxHealth})
	s.World.AddComponent(npc, components.InputComponent{})

	// AI Component
	s.World.AddComponent(npc, components.AIComponent{
		State:        "wander",
		StateTimer:   0,
		Faction:      def.Faction,
		IsAggressive: def.IsAggressive,
		SpawnX:       x,
		SpawnY:       y,
		LeashRange:   600.0, // Stop chasing after 600px
	})

	// Equipment (Weapon)
	if def.WeaponID != "" {
		equip := components.EquipmentComponent{}
		equip.Slots[components.SlotWeapon] = components.EquipmentSlot{ItemID: def.WeaponID}
		s.World.AddComponent(npc, equip)
	}

	// Respawn Component
	s.World.AddComponent(npc, components.RespawnComponent{
		SpawnX:       x,
		SpawnY:       y,
		RespawnTimer: 0,
		IsDead:       false,
	})
}

func (s *GameServer) HandleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	var playerEntity ecs.Entity
	var username string
	var player *Player

	for {
		var packet protocol.Packet
		if err := decoder.Decode(&packet); err != nil {
			log.Printf("Failed to decode auth packet: %v", err)
			return
		}

		if packet.Type == protocol.PacketSignup {
			req := packet.Data.(protocol.SignupPacket)
			if req.Username == "" || req.Password == "" {
				encoder.Encode(protocol.Packet{Type: protocol.PacketSignupResponse, Data: protocol.SignupResponsePacket{Success: false, Error: "Invalid credentials"}})
				continue
			}
			exists, _ := storage.LoadPlayer(req.Username)
			if exists != nil {
				encoder.Encode(protocol.Packet{Type: protocol.PacketSignupResponse, Data: protocol.SignupResponsePacket{Success: false, Error: "User already exists"}})
				continue
			}

			newUser := storage.PlayerSaveData{Username: req.Username, Password: req.Password, X: 100, Y: 100, Health: 100}
			storage.SavePlayer(newUser)
			log.Printf("User signed up: %s", req.Username)
			encoder.Encode(protocol.Packet{Type: protocol.PacketSignupResponse, Data: protocol.SignupResponsePacket{Success: true}})
			continue

		} else if packet.Type == protocol.PacketLogin {
			req := packet.Data.(protocol.LoginPacket)
			saved, err := storage.LoadPlayer(req.Username)

			if err != nil || saved == nil {
				encoder.Encode(protocol.Packet{Type: protocol.PacketLoginResponse, Data: protocol.LoginResponsePacket{Success: false, Error: "User not found"}})
				continue
			}

			if saved.Password != req.Password {
				encoder.Encode(protocol.Packet{Type: protocol.PacketLoginResponse, Data: protocol.LoginResponsePacket{Success: false, Error: "Wrong password"}})
				continue
			}

			username = req.Username
			log.Printf("Player %s logged in", username)

			s.Mutex.Lock()
			playerEntity = s.World.NewEntity()

			spawnX, spawnY := saved.X, saved.Y
			currentHealth := saved.Health

			s.World.AddComponent(playerEntity, components.TransformComponent{X: spawnX, Y: spawnY})
			s.World.AddComponent(playerEntity, components.PhysicsComponent{Speed: 6})
			s.World.AddComponent(playerEntity, components.SpriteComponent{Width: 32, Height: 32, Color: color.RGBA{R: 0, G: 255, B: 0, A: 255}})
			s.World.AddComponent(playerEntity, components.StatsComponent{MaxHealth: 100, CurrentHealth: currentHealth})
			s.World.AddComponent(playerEntity, components.InputComponent{})

			// Initial stats already added above
			// Default weapon stats now fetched dynamically in HandleAttack

			inv := items.NewInventory(25)
			if len(saved.Inventory) > 0 {
				for _, slot := range saved.Inventory {
					if slot.Index >= 0 && slot.Index < 25 {
						inv.Slots[slot.Index].ItemID = slot.ItemID
						inv.Slots[slot.Index].Quantity = slot.Quantity
					}
				}
			} else {
				items.AddItem(inv, "sword_starter", 1)
				items.AddItem(inv, "bow_starter", 1)
				items.AddItem(inv, "potion_red", 5)
			}
			s.World.AddComponent(playerEntity, *inv)

			// Load Hotbar
			var hotbar components.HotbarComponent
			// Restore from save if present
			for i, slot := range saved.Hotbar {
				hotbar.Slots[i] = components.HotbarSlot{
					Type:  slot.Type,
					RefID: slot.RefID,
				}
			}
			s.World.AddComponent(playerEntity, hotbar)

			// Load Equipment
			var equip components.EquipmentComponent
			for i, slot := range saved.Equipment {
				if i < len(equip.Slots) {
					equip.Slots[i].ItemID = slot.ItemID
				}
			}
			s.World.AddComponent(playerEntity, equip)

			spellbook := components.SpellbookComponent{
				UnlockedSpells: saved.UnlockedSpells,
			}
			// Ensure it's not nil slices if possible (JSON might return nil)
			if spellbook.UnlockedSpells == nil {
				spellbook.UnlockedSpells = make([]string, 0)
			}
			s.World.AddComponent(playerEntity, spellbook)

			// Load UI State
			uiState := components.UIStateComponent{
				OpenMenus: saved.OpenMenus,
			}
			if uiState.OpenMenus == nil {
				uiState.OpenMenus = make(map[string]bool)
			}
			s.World.AddComponent(playerEntity, uiState)

			keybindings := saved.Keybindings
			if keybindings == nil {
				keybindings = make(map[string]int)
			}
			// Merge Defaults (Ensure new keys like "Spells" are present)
			// KeyM = 12 (A=0, ..., I=8, ..., M=12)
			defaults := map[string]int{
				"Spells": 12,
			}
			for k, v := range defaults {
				if _, exists := keybindings[k]; !exists {
					keybindings[k] = v
				}
			}

			player = &Player{
				Conn:     conn,
				Encoder:  encoder,
				Decoder:  decoder,
				EntityID: playerEntity,
				Username: username,
			}
			s.Players[playerEntity] = player
			s.Mutex.Unlock()

			response := protocol.Packet{
				Type: protocol.PacketLoginResponse,
				Data: protocol.LoginResponsePacket{
					Success:        true,
					PlayerEntityID: playerEntity,
					PlayerX:        spawnX,
					PlayerY:        spawnY,
					MapWidth:       s.Maps[0].Width,
					MapHeight:      s.Maps[0].Height,
					MapTiles:       world.FlattenTiles(s.Maps[0].Tiles),
					MapObjects:     world.FlattenObjects(s.Maps[0].Objects),
					UnlockedSpells: saved.UnlockedSpells,
					Keybindings:    keybindings,
					DebugSettings:  saved.DebugSettings,
					OpenMenus:      saved.OpenMenus,
				},
			}
			if err := encoder.Encode(response); err != nil {
				log.Printf("Failed to send login response: %v", err)
				s.RemovePlayer(playerEntity)
				return
			}

			s.SendInventorySync(player)
			s.SendHotbarSync(player)
			s.SendEquipmentSync(player)
			s.SendMapSync(player)
			break
		}
	}

	for {
		var packet protocol.Packet
		if err := decoder.Decode(&packet); err != nil {
			log.Printf("Player %d disconnected: %v", playerEntity, err)
			s.RemovePlayer(playerEntity)
			return
		}
		if packet.Type == protocol.PacketInput {
			input := packet.Data.(protocol.InputPacket)
			s.ProcessInput(playerEntity, input.Input)
		} else if packet.Type == protocol.PacketUpdateKeybindings {
			data := packet.Data.(protocol.UpdateKeybindingsPacket)
			s.Mutex.Lock()
			currData, err := storage.LoadPlayer(username)
			if err == nil && currData != nil {
				currData.Keybindings = data.Keybindings
				storage.SavePlayer(*currData)
				log.Printf("Updated keybindings for %s", username)
			}
			s.Mutex.Unlock()
		} else if packet.Type == protocol.PacketInventoryAction {
			// Handle Inventory Actions
			// Move this to InventorySystem later
			action := packet.Data.(protocol.InventoryActionPacket)
			s.HandleInventoryAction(playerEntity, action, player)
		} else if packet.Type == protocol.PacketHotbarAction {
			action := packet.Data.(protocol.HotbarActionPacket)
			s.HandleHotbarAction(playerEntity, action, player)
		} else if packet.Type == protocol.PacketEquipmentAction {
			action := packet.Data.(protocol.EquipmentActionPacket)
			s.HandleEquipmentAction(playerEntity, action, player)
		} else if packet.Type == protocol.PacketCastSpell {
			req := packet.Data.(protocol.CastSpellPacket)
			s.Mutex.Lock()
			// Use cursor position from last known input for target?
			// Or just assume self/direction?
			// Instants like Heal are self. Blink is directional.
			// InputComponent has MouseX/Y.
			var mx, my float64
			if input, ok := ecs.GetComponent[components.InputComponent](s.World, playerEntity); ok {
				mx, my = input.MouseX, input.MouseY
			}
			// We can pass this to handler
			s.handleSpellCast(playerEntity, req.SpellID, mx, my)
			s.Mutex.Unlock()
		} else if packet.Type == protocol.PacketUpdateUIState {
			data := packet.Data.(protocol.UpdateUIStatePacket)
			s.Mutex.Lock()
			uiState, _ := ecs.GetComponent[components.UIStateComponent](s.World, playerEntity)
			if uiState == nil {
				uiState = &components.UIStateComponent{OpenMenus: make(map[string]bool)}
			}
			// Update state
			uiState.OpenMenus = data.OpenMenus
			s.World.AddComponent(playerEntity, *uiState)
			// Save
			if err := s.PersistenceSystem.SavePlayer(playerEntity, username); err != nil {
				log.Printf("Error saving UI state: %v", err)
			}
			s.Mutex.Unlock()
		}
	}
}

func (s *GameServer) HandleInventoryAction(id ecs.Entity, action protocol.InventoryActionPacket, player *Player) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	inv, _ := ecs.GetComponent[components.InventoryComponent](s.World, id)
	if inv == nil {
		return
	}

	if action.ActionType == "Swap" {
		items.SwapItems(inv, action.SlotA, action.SlotB)
	} else if action.ActionType == "Drop" {
		// Remove item from slot
		// For now, just delete. Future: Spawn drop entity.
		if action.SlotA >= 0 && action.SlotA < len(inv.Slots) {
			inv.Slots[action.SlotA].ItemID = ""
			inv.Slots[action.SlotA].Quantity = 0
			log.Printf("Player %s dropped item from slot %d", player.Username, action.SlotA)
		}
	} else if action.ActionType == "Primary" {
		if action.SlotA >= 0 && action.SlotA < len(inv.Slots) {
			itemID := inv.Slots[action.SlotA].ItemID
			if itemID != "" {
				def, ok := items.Get(itemID)
				if ok && def.EquipmentSlot != -1 {
					s.equipItemInternal(id, action.SlotA, def.EquipmentSlot, player)
					return
				}
				// Handle Consumables here later
				log.Printf("Player %s used primary action on slot %d: %s", player.Username, action.SlotA, itemID)
			}
		}
	}
	// Save changes back to World
	s.World.AddComponent(id, *inv)

	// Explicitly save to file
	go s.PersistenceSystem.SavePlayer(id, player.Username)

	// Sync inventory change back to client
	go s.SendInventorySync(player)
}

func (s *GameServer) HandleEquipmentAction(id ecs.Entity, action protocol.EquipmentActionPacket, player *Player) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	if action.Action == "Equip" {
		s.equipItemInternal(id, action.InvSlot, action.Slot, player)
	} else if action.Action == "Unequip" {
		equip, _ := ecs.GetComponent[components.EquipmentComponent](s.World, id)
		inv, _ := ecs.GetComponent[components.InventoryComponent](s.World, id)

		if equip == nil || inv == nil {
			return
		}

		if action.Slot < 0 || action.Slot >= 9 {
			return
		}
		itemID := equip.Slots[action.Slot].ItemID
		if itemID == "" {
			return
		}

		// Try to add to Inventory
		err := items.AddItem(inv, itemID, 1)
		if err == nil {
			equip.Slots[action.Slot].ItemID = ""
			log.Printf("Player %s unequipped %s", player.Username, itemID)
		} else {
			log.Printf("Player %s failed to unequip %s: Inventory Full", player.Username, itemID)
		}

		// Save components explicitly!
		s.World.AddComponent(id, *equip)
		s.World.AddComponent(id, *inv)

		go s.SendInventorySync(player)
		go s.SendEquipmentSync(player)
	}

	// Explicitly save to file after any equipment change
	go s.PersistenceSystem.SavePlayer(id, player.Username)
}

func (s *GameServer) HandleHotbarAction(id ecs.Entity, action protocol.HotbarActionPacket, player *Player) {
	s.Mutex.Lock()
	// defer s.Mutex.Unlock() // REMOVED to avoid double unlock

	hb, _ := ecs.GetComponent[components.HotbarComponent](s.World, id)
	if hb == nil {
		s.Mutex.Unlock()
		return
	}

	if action.ActionType == "Bind" {
		if action.SlotIndex >= 0 && action.SlotIndex < 10 {
			hb.Slots[action.SlotIndex].Type = action.TargetType
			hb.Slots[action.SlotIndex].RefID = action.TargetRefID
			log.Printf("Player %s bound %s:%s to slot %d", player.Username, action.TargetType, action.TargetRefID, action.SlotIndex)
		}
	} else if action.ActionType == "Swap" {
		if action.SlotIndex >= 0 && action.SlotIndex < 10 && action.SlotIndexB >= 0 && action.SlotIndexB < 10 {
			hb.Slots[action.SlotIndex], hb.Slots[action.SlotIndexB] = hb.Slots[action.SlotIndexB], hb.Slots[action.SlotIndex]
		}
	}

	// Save back to world
	s.World.AddComponent(id, *hb)

	// Explicitly save to file
	go s.PersistenceSystem.SavePlayer(id, player.Username)

	s.Mutex.Unlock()

	// Sync back to client
	s.SendHotbarSync(player)
}

func (s *GameServer) RemovePlayer(id ecs.Entity) {
	s.Mutex.Lock()

	if player, ok := s.Players[id]; ok {
		// Use Persistence System
		if err := s.PersistenceSystem.SavePlayer(id, player.Username); err != nil {
			log.Printf("Failed to save player %s: %v", player.Username, err)
		}
	}

	delete(s.Players, id)
	s.World.RemoveEntity(id)
	s.Mutex.Unlock()
}

func (s *GameServer) ProcessInput(id ecs.Entity, input components.InputComponent) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	player, ok := s.Players[id]
	if !ok {
		return
	}

	if input.Attack {
		// Log attack?
	}

	// Handle Hotbar Triggers
	hb, _ := ecs.GetComponent[components.HotbarComponent](s.World, id)
	if hb != nil {
		for i := 0; i < 10; i++ {
			if input.HotbarTriggers[i] && !player.PrevInput.HotbarTriggers[i] {
				slot := hb.Slots[i]
				if slot.Type == "Item" && slot.RefID != "" {
					s.toggleEquipItem(id, slot.RefID, player)
				} else if slot.Type == "Spell" && slot.RefID != "" {
					// Toggle Active Spell if Combat, or Cast if Instant
					def, exists := components.SpellRegistry[slot.RefID]
					if exists {
						if def.Type == "combat" {
							// For hotbar toggling active spell:
							// Server doesn't assume UI state (Client UI handles "Selection").
							// BUT: Player pressed Hotkey. Server receives `HotbarTriggers[i]`.
							// What should server do?
							// Option A: Server tells Client "Select spell X".
							// Option B: Client InputSystem handles hotbar press locally -> Selects Spell -> Sends Input ActiveSpell.
							// Currently `ProcessInput` just consumes trigger.

							// Correct approach: Client InputSystem should intercept Hotbar key and update ActiveSpell LOCALLY if it's a spell.
							// Server only needs to know about "Inventory/Equip" hotbar actions perhaps?
							// Or if Hotbar is fully server-side, Server needs to send "State Update" that changes active spell?

							// If I implemented `ActiveSpell` as INPUT field, then Client controls it.
							// So Client should handle Hotbar press for Spells.
							// `InputSystem.Update` sends `HotbarTriggers` to server.
							// But `InputSystem` doesn't check what's IN the hotbar slot.

							// Solution: `UISystem` should handle Hotbar for "Selection" locally?
							// `pkg/client/systems/input.go` loop 69-74 checks keys.
							// If `ActiveSpell` is purely client-side state fed to input, Client needs to know hotbar contents.
							// `UISystem` has `BindWidget` which has slots.
							// `UISystem` is best place.

							// Let's NOT modify Server ProcessInput for spells if Client handles it.
							// BUT `InputSystem` sends triggers. Server executes "Use Item".
							// For Spells, Server can't easily "Select" it for the client UI.

							// Let's modify Client `HandleGlobalKeys` or `InputSystem` to check Hotbar usage.
						} else {
							// Instant Cast via Hotbar (Server side logic ok)
							// Get mouse pos from Input
							s.handleSpellCast(id, slot.RefID, input.MouseX, input.MouseY)
						}
					}
				}
			}
		}
	}

	s.World.AddComponent(id, input)
}

func (s *GameServer) GameLoop() {
	ticker := time.NewTicker(time.Millisecond * 33) // ~30 TPS
	defer ticker.Stop()

	for range ticker.C {
		s.Update()
		s.BroadcastState()
	}
}

func (s *GameServer) UpdateRespawn(dt float64) {
	respawners := ecs.Query[components.RespawnComponent](s.World)
	for _, id := range respawners {
		respawn, _ := ecs.GetComponent[components.RespawnComponent](s.World, id)
		if respawn == nil || !respawn.IsDead {
			continue
		}

		respawn.RespawnTimer -= dt
		if respawn.RespawnTimer <= 0 {
			// RESPAWN!
			respawn.IsDead = false
			s.World.AddComponent(id, *respawn)

			// Restore Components
			s.World.AddComponent(id, components.TransformComponent{X: respawn.SpawnX, Y: respawn.SpawnY})
			s.World.AddComponent(id, components.PhysicsComponent{Speed: 6})
			s.World.AddComponent(id, components.SpriteComponent{Width: 32, Height: 32, Color: color.RGBA{R: 255, G: 255, B: 0, A: 255}})
			s.World.AddComponent(id, components.StatsComponent{MaxHealth: 50, CurrentHealth: 50})
			s.World.AddComponent(id, components.InputComponent{})
			s.World.AddComponent(id, components.AIComponent{
				Type:         "wander",
				State:        "wander", // Start wandering, not idle, to ensure logic kicks in
				StateTimer:   1.0,
				IsAggressive: true, // Guards are aggressive
				Faction:      1,
				SpawnX:       respawn.SpawnX,
				SpawnY:       respawn.SpawnY,
				LeashRange:   600.0,
			})
			log.Printf("Entity %d respawned at %.1f, %.1f", id, respawn.SpawnX, respawn.SpawnY)
		} else {
			s.World.AddComponent(id, *respawn)
		}
	}
}

func (s *GameServer) Update() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	// Update AI
	s.AISystem.Update(0.033)

	// Update Deads/Respawn
	s.UpdateRespawn(0.033)

	// Move Players/NPCs via System
	s.MovementSystem.Update(0.033)

	// Handle Attacks for ALL entities with Input (Players AND NPCs)
	inputs := ecs.Query[components.InputComponent](s.World)
	for _, id := range inputs {
		s.HandleAttack(id)
	}

	for id, player := range s.Players {
		if input, ok := ecs.GetComponent[components.InputComponent](s.World, id); ok {
			player.PrevInput = *input
		}
	}

	projectiles := ecs.Query[components.ProjectileComponent](s.World)
	for _, pid := range projectiles {
		s.UpdateProjectile(pid)
	}

	s.World.Update(0.033)
}

func (s *GameServer) HandleAttack(id ecs.Entity) {
	input, _ := ecs.GetComponent[components.InputComponent](s.World, id)

	// Check cooldown logic first
	if input == nil || !input.Attack {
		return
	}

	// For Players, we prevent spamming by checking PrevInput (button press vs hold)
	// For AI, they control the boolean directly, so we can trust it (or they implement their own hold logic)
	if player, isPlayer := s.Players[id]; isPlayer {
		if player.PrevInput.Attack {
			return
		}
	} else {
		// NPC Logic: AI system sets Attack=true for one frame when it wants to attack?
		// Or we implement auto-reset of Attack flag in AI system?
		// Actually, InputComponent persists. GameServer Update loop copies input to prevInput for players.
		// For NPCs, who updates PrevInput? Nobody currently.
		// Let's rely on AttackComponent cooldown to limit attack rate, which is robust.
	}

	// 1. Check Active Spell (High Priority)
	if input.ActiveSpell != "" {
		s.handleSpellCast(id, input.ActiveSpell, input.MouseX, input.MouseY)
		return
	}

	// 2. Fetch Dynamic Stats from Equipment (Fallback to Weapon)
	var damage, attackRange, cooldown float64
	var attackType components.AttackType

	equip, _ := ecs.GetComponent[components.EquipmentComponent](s.World, id)
	weaponFound := false
	if equip != nil {
		weaponID := equip.Slots[components.SlotWeapon].ItemID
		if weaponID != "" {
			if def, ok := items.Get(weaponID); ok && def.WeaponStats != nil {
				damage = def.WeaponStats.Damage
				attackRange = def.WeaponStats.Range
				cooldown = def.WeaponStats.Cooldown
				attackType = def.WeaponStats.Type
				weaponFound = true
			}
		}
	}

	if !weaponFound {
		return
	}

	// 3. Use AttackComponent ONLY for LastAttackTime tracking
	attackComp, _ := ecs.GetComponent[components.AttackComponent](s.World, id)
	if attackComp == nil {
		attackComp = &components.AttackComponent{}
		// Initialize to allow immediate attack
	}

	now := float64(time.Now().UnixMilli()) / 1000.0
	if now-attackComp.LastAttackTime < cooldown {
		return
	}

	transform, _ := ecs.GetComponent[components.TransformComponent](s.World, id)
	if transform == nil {
		return
	}

	// Update Cooldown State
	attackComp.LastAttackTime = now
	s.World.AddComponent(id, *attackComp)

	// 3. Spawn Projectile from Dynamic Center (Calculate once for all types)
	// Default Size
	width, height := 32.0, 32.0
	if sprite, ok := ecs.GetComponent[components.SpriteComponent](s.World, id); ok {
		width = float64(sprite.Width)
		height = float64(sprite.Height)
	}

	startX := transform.X + width/2
	startY := transform.Y + height/2

	if attackType == components.AttackTypeRanged {
		proj := s.World.NewEntity()
		// Direction from CENTER to Mouse
		dirX, dirY := components.Direction(startX, startY, input.MouseX, input.MouseY)

		speed := 10.0
		lifetime := attackRange / speed

		spawnDist := 16.0 // Spawn at edge of character circle
		spawnX := startX + dirX*spawnDist
		spawnY := startY + dirY*spawnDist

		s.World.AddComponent(proj, components.TransformComponent{X: spawnX, Y: spawnY})
		s.World.AddComponent(proj, components.PhysicsComponent{VelX: dirX * speed, VelY: dirY * speed, Speed: speed})
		s.World.AddComponent(proj, components.SpriteComponent{Width: 8, Height: 8, Color: color.RGBA{R: 255, G: 255, B: 0, A: 255}})
		s.World.AddComponent(proj, components.ProjectileComponent{OwnerID: id, Damage: damage, Lifetime: lifetime})

	} else if attackType == components.AttackTypeMelee {
		slash := s.World.NewEntity()
		dirX, dirY := components.Direction(transform.X, transform.Y, input.MouseX, input.MouseY)
		offsetX := dirX * 30
		offsetY := dirY * 30

		s.World.AddComponent(slash, components.TransformComponent{X: transform.X + offsetX, Y: transform.Y + offsetY})
		s.World.AddComponent(slash, components.SpriteComponent{Width: 40, Height: 40, Color: color.RGBA{R: 255, G: 0, B: 0, A: 255}})
		s.World.AddComponent(slash, components.ProjectileComponent{OwnerID: id, Damage: damage, Lifetime: 15}) // Melee slash duration in ticks
	}
}

func (s *GameServer) UpdateProjectile(pid ecs.Entity) {
	transform, _ := ecs.GetComponent[components.TransformComponent](s.World, pid)
	proj, _ := ecs.GetComponent[components.ProjectileComponent](s.World, pid)
	phys, _ := ecs.GetComponent[components.PhysicsComponent](s.World, pid)

	if transform == nil || proj == nil {
		return
	}

	if phys != nil {
		transform.X += phys.VelX
		transform.Y += phys.VelY
	}

	proj.Lifetime -= 1
	if proj.Lifetime <= 0 {
		s.World.RemoveEntity(pid)
		return
	}

	s.World.AddComponent(pid, *transform)
	s.World.AddComponent(pid, *proj)

	// terrain Collision (Projectiles)
	// Check center of projectile
	cx := transform.X + 4
	cy := transform.Y + 4
	tx := int(cx / 32.0)
	ty := int(cy / 32.0)

	// Projectile Z
	z := transform.Z
	if m, ok := s.Maps[z]; ok {
		if tx >= 0 && tx < m.Width && ty >= 0 && ty < m.Height {
			tile := m.Tiles[ty][tx]
			if tile.Type == world.TileTree || m.Objects[ty][tx] > 0 {
				// Tree/Object is solid -> Block
				s.World.RemoveEntity(pid)
				return
			}
			// If Water, we DO NOT destroy.
		}
	}

	// Collision Detection
	// Simple O(N) check against all entities with Stats (Health)
	targets := ecs.Query[components.StatsComponent](s.World)
	projRect := struct{ X, Y, W, H float64 }{transform.X, transform.Y, 10, 10}
	// Assuming projectile size for collision

	for _, tid := range targets {
		if tid == proj.OwnerID {
			continue // Don't hit yourself
		}

		targetStats, _ := ecs.GetComponent[components.StatsComponent](s.World, tid)
		targetTrans, _ := ecs.GetComponent[components.TransformComponent](s.World, tid)
		targetSprite, _ := ecs.GetComponent[components.SpriteComponent](s.World, tid)

		if targetTrans == nil || targetSprite == nil {
			continue
		}

		// AABB Check
		if s.rectOverlap(projRect.X, projRect.Y, projRect.W, projRect.H,
			targetTrans.X, targetTrans.Y, targetSprite.Width, targetSprite.Height) {

			// HIT!
			targetStats.CurrentHealth -= proj.Damage
			if targetStats.CurrentHealth < 0 {
				targetStats.CurrentHealth = 0 // Clamp Health
			}
			s.World.AddComponent(tid, *targetStats)

			log.Printf("Entity %d hit Entity %d for %.1f damage (HP: %.1f)", proj.OwnerID, tid, proj.Damage, targetStats.CurrentHealth)

			// Check Death
			if targetStats.CurrentHealth <= 0 {
				if respawn, ok := ecs.GetComponent[components.RespawnComponent](s.World, tid); ok {
					respawn.IsDead = true
					respawn.RespawnTimer = 30.0
					s.World.AddComponent(tid, *respawn)

					// Despawn (Remove components)
					s.World.RemoveComponent(tid, components.SpriteComponent{})
					s.World.RemoveComponent(tid, components.PhysicsComponent{})
					s.World.RemoveComponent(tid, components.AIComponent{})
					s.World.RemoveComponent(tid, components.InputComponent{})
					s.World.RemoveComponent(tid, components.StatsComponent{})
					s.World.RemoveComponent(tid, components.TransformComponent{})

					log.Printf("Entity %d died. Respawning in 30s.", tid)
				}
			} else {
				// Aggro Logic: If victim is alive and NPC, set target to attacker
				if ai, ok := ecs.GetComponent[components.AIComponent](s.World, tid); ok {
					if ai.TargetID == 0 {
						ai.TargetID = proj.OwnerID
						ai.State = "chase"
						s.World.AddComponent(tid, *ai)
						log.Printf("Entity %d is now chasing Entity %d", tid, proj.OwnerID)
					}
				}
			}

			// Destroy Projectile
			s.World.RemoveEntity(pid)
			return // One hit per projectile
		}
	}
}

func (s *GameServer) rectOverlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func (s *GameServer) BroadcastState() {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	packet := s.NetworkSystem.PrepareStateUpdate()

	for _, p := range s.Players {
		go func(player *Player) {
			if err := player.Encoder.Encode(packet); err != nil {
				// handled
			}
		}(p)
	}
}

func (s *GameServer) SendInventorySync(player *Player) {
	s.Mutex.RLock()
	inv, _ := ecs.GetComponent[components.InventoryComponent](s.World, player.EntityID)
	s.Mutex.RUnlock()

	if inv == nil {
		return
	}

	syncSlots := make([]struct {
		Index    int
		ItemID   string
		Quantity int
	}, 0)
	for i, slot := range inv.Slots {
		if slot.ItemID != "" && slot.Quantity > 0 {
			syncSlots = append(syncSlots, struct {
				Index    int
				ItemID   string
				Quantity int
			}{
				Index:    i,
				ItemID:   slot.ItemID,
				Quantity: slot.Quantity,
			})
		}
	}

	packet := protocol.Packet{
		Type: protocol.PacketInventorySync,
		Data: protocol.InventorySyncPacket{
			Slots:    syncSlots,
			Capacity: inv.Capacity,
		},
	}

	if err := player.Encoder.Encode(packet); err != nil {
		log.Printf("Failed to send inventory sync: %v", err)
	}
}

func (s *GameServer) SendHotbarSync(player *Player) {
	s.Mutex.RLock()
	hb, _ := ecs.GetComponent[components.HotbarComponent](s.World, player.EntityID)
	s.Mutex.RUnlock()

	if hb == nil {
		return
	}

	var syncPacket protocol.HotbarSyncPacket
	for i, slot := range hb.Slots {
		syncPacket.Slots[i] = protocol.HotbarSyncSlot{
			Type:  slot.Type,
			RefID: slot.RefID,
		}
	}

	// Debug Log
	log.Printf("Sending HotbarSync to %s: %v", player.Username, syncPacket.Slots)

	packet := protocol.Packet{
		Type: protocol.PacketHotbarSync,
		Data: syncPacket,
	}

	if err := player.Encoder.Encode(packet); err != nil {
		log.Printf("Failed to send hotbar sync: %v", err)
	}
}

func (s *GameServer) SendEquipmentSync(player *Player) {
	s.Mutex.RLock()
	equip, _ := ecs.GetComponent[components.EquipmentComponent](s.World, player.EntityID)
	s.Mutex.RUnlock()

	if equip == nil {
		return
	}

	var syncPacket protocol.EquipmentSyncPacket
	for i, slot := range equip.Slots {
		syncPacket.Slots[i].ItemID = slot.ItemID
	}

	packet := protocol.Packet{
		Type: protocol.PacketEquipmentSync,
		Data: syncPacket,
	}

	if err := player.Encoder.Encode(packet); err != nil {
		log.Printf("Failed to send equipment sync: %v", err)
	}
}

// equipItemInternal performs the actual equip logic. Assumes s.Mutex is LOCKED.
func (s *GameServer) equipItemInternal(id ecs.Entity, invSlot int, equipSlot int, player *Player) {
	equip, _ := ecs.GetComponent[components.EquipmentComponent](s.World, id)
	inv, _ := ecs.GetComponent[components.InventoryComponent](s.World, id)

	if equip == nil || inv == nil {
		return
	}

	// Verify Inventory Slot
	if invSlot < 0 || invSlot >= len(inv.Slots) {
		return
	}
	itemID := inv.Slots[invSlot].ItemID
	if itemID == "" {
		return
	}

	// Verify Item Type and Target Slot
	def, ok := items.Get(itemID)
	if !ok || def.EquipmentSlot == -1 {
		log.Printf("Player %s tried to equip non-equippable item %s", player.Username, itemID)
		return
	}
	if def.EquipmentSlot != equipSlot {
		log.Printf("Player %s tried to equip %s to wrong slot %d (expected %d)", player.Username, itemID, equipSlot, def.EquipmentSlot)
		return
	}

	// Perform Swap
	// 1. Take from Inventory (assuming equipment items stack to 1 generally, but handle quantity)
	inv.Slots[invSlot].Quantity--
	if inv.Slots[invSlot].Quantity <= 0 {
		inv.Slots[invSlot].ItemID = ""
		inv.Slots[invSlot].Quantity = 0
	}

	// 2. Check if Equipment Slot has item (Swap)
	oldItem := equip.Slots[equipSlot].ItemID
	equip.Slots[equipSlot].ItemID = itemID

	// 3. Return old item to inventory
	if oldItem != "" {
		if inv.Slots[invSlot].ItemID == "" {
			inv.Slots[invSlot].ItemID = oldItem
			inv.Slots[invSlot].Quantity = 1
		} else {
			err := items.AddItem(inv, oldItem, 1)
			if err != nil {
				// Revert
				equip.Slots[equipSlot].ItemID = oldItem
				items.AddItem(inv, itemID, 1)
				log.Printf("Inventory full, could not unequip old item %s", oldItem)
				return
			}
		}
	}

	log.Printf("Player %s equipped %s to slot %d", player.Username, itemID, equipSlot)

	// Save components explicitly!
	s.World.AddComponent(id, *equip)
	s.World.AddComponent(id, *inv)

	go s.SendInventorySync(player)
	go s.SendEquipmentSync(player)
}

// toggleEquipItem toggles an item between equipped and inventory states. Assumes s.Mutex is LOCKED.
func (s *GameServer) toggleEquipItem(id ecs.Entity, itemID string, player *Player) {
	equip, _ := ecs.GetComponent[components.EquipmentComponent](s.World, id)
	inv, _ := ecs.GetComponent[components.InventoryComponent](s.World, id)

	if equip == nil || inv == nil {
		return
	}

	// 1. Check if already equipped
	foundSlot := -1
	for i, slot := range equip.Slots {
		if slot.ItemID == itemID {
			foundSlot = i
			break
		}
	}

	if foundSlot != -1 {
		// ALREADY EQUIPPED -> UNEQUIP
		// Try to add back to inventory
		err := items.AddItem(inv, itemID, 1)
		if err == nil {
			equip.Slots[foundSlot].ItemID = ""
			log.Printf("Player %s unequipped %s via hotbar", player.Username, itemID)
		} else {
			log.Printf("Player %s failed to unequip %s via hotbar: Inventory full", player.Username, itemID)
		}
	} else {
		// NOT EQUIPPED -> EQUIP
		// Find in inventory
		invSlot := -1
		for i, slot := range inv.Slots {
			if slot.ItemID == itemID {
				invSlot = i
				break
			}
		}

		if invSlot != -1 {
			def, ok := items.Get(itemID)
			if ok && def.EquipmentSlot != -1 {
				s.equipItemInternal(id, invSlot, def.EquipmentSlot, player)
			}
		} else {
			log.Printf("Player %s tried to hotbar equip %s but it's not in inventory", player.Username, itemID)
		}
	}

	// Sync happens inside unequip logic or equipItemInternal
	// But for unequip we need to sync manually here if we changed it.
	if foundSlot != -1 {
		s.World.AddComponent(id, *equip)
		s.World.AddComponent(id, *inv)
		go s.SendInventorySync(player)
		go s.SendEquipmentSync(player)
	}
}

func (s *GameServer) SendMapSync(player *Player) {
	// Determine which map to send
	// For now, assume player is on Level 0 if not set, or fetch from Transform
	trans, _ := ecs.GetComponent[components.TransformComponent](s.World, player.EntityID)
	z := 0
	if trans != nil {
		z = trans.Z
	}

	gameMap, ok := s.Maps[z]
	if !ok {
		return // No map to sync?
	}

	// Flatten Tiles and Objects
	tiles := make([]int, gameMap.Width*gameMap.Height)
	objects := make([]int, gameMap.Width*gameMap.Height)
	for y := 0; y < gameMap.Height; y++ {
		for x := 0; x < gameMap.Width; x++ {
			tiles[y*gameMap.Width+x] = int(gameMap.Tiles[y][x].Type)
			objects[y*gameMap.Width+x] = gameMap.Objects[y][x]
		}
	}

	packet := protocol.Packet{
		Type: protocol.PacketMapSync,
		Data: protocol.MapSyncPacket{
			Level:   z,
			Width:   gameMap.Width,
			Height:  gameMap.Height,
			Tiles:   tiles,
			Objects: objects,
		},
	}
	player.Encoder.Encode(packet)
}

func (s *GameServer) handleSpellCast(id ecs.Entity, spellID string, targetX, targetY float64) {
	// Verify Unlock
	spellbook, _ := ecs.GetComponent[components.SpellbookComponent](s.World, id)
	if spellbook == nil {
		return
	}

	unlocked := false
	if spellbook.UnlockedSpells != nil {
		for _, s := range spellbook.UnlockedSpells {
			if s == spellID {
				unlocked = true
				break
			}
		}
	}
	if !unlocked {
		return
	}

	// Verify Cooldown
	if spellbook.Cooldowns == nil {
		spellbook.Cooldowns = make(map[string]float64)
	}

	now := float64(time.Now().UnixMilli()) / 1000.0
	lastCast := spellbook.Cooldowns[spellID]

	spellDef, exists := components.SpellRegistry[spellID]
	if !exists {
		return
	}

	if now-lastCast < spellDef.Cooldown {
		return // On Cooldown
	}

	// Cast Spell
	spellbook.Cooldowns[spellID] = now
	s.World.AddComponent(id, *spellbook)

	// Notify Client of Cooldown (Sync)
	if player, ok := s.Players[id]; ok {
		go s.SendSpellbookSync(player)
	}

	// Logic
	transform, _ := ecs.GetComponent[components.TransformComponent](s.World, id)
	if transform == nil {
		return
	}

	if spellID == "fireball" {
		// Projectile
		proj := s.World.NewEntity()
		dirX, dirY := components.Direction(transform.X, transform.Y, targetX, targetY)
		speed := 12.0
		damage := 25.0
		lifetime := 60.0 // 2 seconds (30 TPS)

		spawnDist := 20.0
		spawnX := transform.X + dirX*spawnDist
		spawnY := transform.Y + dirY*spawnDist

		s.World.AddComponent(proj, components.TransformComponent{X: spawnX, Y: spawnY})
		s.World.AddComponent(proj, components.PhysicsComponent{VelX: dirX * speed, VelY: dirY * speed, Speed: speed})
		s.World.AddComponent(proj, components.SpriteComponent{Width: 12, Height: 12, Color: spellDef.Color})
		s.World.AddComponent(proj, components.ProjectileComponent{OwnerID: id, Damage: damage, Lifetime: lifetime})

	} else if spellID == "heal" {
		stats, _ := ecs.GetComponent[components.StatsComponent](s.World, id)
		if stats != nil {
			stats.CurrentHealth += 20
			if stats.CurrentHealth > stats.MaxHealth {
				stats.CurrentHealth = stats.MaxHealth
			}
			s.World.AddComponent(id, *stats)
			log.Printf("Entity %d healed. HP: %.1f", id, stats.CurrentHealth)
		}
	} else if spellID == "blink" {
		dirX, dirY := components.Direction(transform.X, transform.Y, targetX, targetY)
		dist := 100.0
		// Should check collision?
		transform.X += dirX * dist
		transform.Y += dirY * dist
		s.World.AddComponent(id, *transform)
	}
	// Add other spells...
}

func (s *GameServer) SendSpellbookSync(player *Player) {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	sb, _ := ecs.GetComponent[components.SpellbookComponent](s.World, player.EntityID)
	if sb == nil {
		return
	}

	packet := protocol.Packet{
		Type: protocol.PacketSpellbookSync,
		Data: protocol.SpellbookSyncPacket{
			UnlockedSpells: sb.UnlockedSpells,
			Cooldowns:      sb.Cooldowns,
		},
	}
	player.Encoder.Encode(packet)
}
