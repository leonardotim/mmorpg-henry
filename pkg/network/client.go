package network

import (
	"encoding/gob"
	"fmt"
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
	"henry/pkg/shared/network"
	"henry/pkg/shared/world"
	"log"
	"net"
	"sync"
)

type NetworkClient struct {
	Conn           net.Conn
	Encoder        *gob.Encoder
	Decoder        *gob.Decoder
	PlayerEntityID ecs.Entity
	State          network.StateUpdatePacket
	Inventory      network.InventorySyncPacket
	Hotbar         network.HotbarSyncPacket
	Equipment      network.EquipmentSyncPacket
	Map            network.MapSyncPacket
	WorldMap       *world.Map
	UnlockedSpells []string
	Cooldowns      map[string]float64
	Mutex          sync.RWMutex
}

func (c *NetworkClient) GetEquipment() network.EquipmentSyncPacket {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.Equipment
}

func NewNetworkClient() *NetworkClient {
	return &NetworkClient{}
}

func (c *NetworkClient) Signup(address, username, password string) error {
	conn, err := Dial(address)
	if err != nil {
		return err
	}
	defer conn.Close()

	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	// Send Signup
	req := network.Packet{
		Type: network.PacketSignup,
		Data: network.SignupPacket{Username: username, Password: password},
	}
	if err := enc.Encode(req); err != nil {
		return err
	}

	// Wait for Response
	var response network.Packet
	if err := dec.Decode(&response); err != nil {
		return err
	}

	if response.Type != network.PacketSignupResponse {
		return fmt.Errorf("unexpected packet: %d", response.Type)
	}

	respData := response.Data.(network.SignupResponsePacket)
	if !respData.Success {
		return fmt.Errorf("signup failed: %s", respData.Error)
	}

	return nil
}

func (c *NetworkClient) Connect(address, username, password string) (map[string]int, map[string]bool, map[string]bool, bool, error) {
	conn, err := Dial(address)
	if err != nil {
		return nil, nil, nil, false, err
	}

	c.Conn = conn
	c.Encoder = gob.NewEncoder(conn)
	c.Decoder = gob.NewDecoder(conn)

	// Send Login
	login := network.Packet{
		Type: network.PacketLogin,
		Data: network.LoginPacket{Username: username, Password: password},
	}
	if err := c.Encoder.Encode(login); err != nil {
		return nil, nil, nil, false, err
	}

	// Wait for Login Response
	var response network.Packet
	if err := c.Decoder.Decode(&response); err != nil {
		return nil, nil, nil, false, err
	}
	if response.Type != network.PacketLoginResponse {
		return nil, nil, nil, false, fmt.Errorf("unexpected packet type: %d", response.Type)
	}

	respData := response.Data.(network.LoginResponsePacket)
	if !respData.Success {
		return nil, nil, nil, false, fmt.Errorf("login failed: %s", respData.Error)
	}

	c.PlayerEntityID = respData.PlayerEntityID
	log.Printf("Logged in. EntityID: %d", c.PlayerEntityID)

	// Init Map
	c.WorldMap = &world.Map{
		Width:   respData.MapWidth,
		Height:  respData.MapHeight,
		Tiles:   world.UnflattenTiles(respData.MapTiles, respData.MapWidth, respData.MapHeight),
		Objects: world.UnflattenObjects(respData.MapObjects, respData.MapWidth, respData.MapHeight),
	}
	c.UnlockedSpells = respData.UnlockedSpells

	// Start listening loop
	go c.ListenLoop()
	return respData.Keybindings, respData.DebugSettings, respData.OpenMenus, respData.IsRunning, nil
}

func (c *NetworkClient) ListenLoop() {
	for {
		var packet network.Packet
		if err := c.Decoder.Decode(&packet); err != nil {
			log.Printf("Disconnected from server: %v", err)
			return
		}

		if packet.Type == network.PacketStateUpdate {
			state := packet.Data.(network.StateUpdatePacket)
			c.Mutex.Lock()
			c.State = state
			c.Mutex.Unlock()
		} else if packet.Type == network.PacketInventorySync {
			inv := packet.Data.(network.InventorySyncPacket)
			c.Mutex.Lock()
			c.Inventory = inv
			c.Mutex.Unlock()
		} else if packet.Type == network.PacketHotbarSync {
			hb := packet.Data.(network.HotbarSyncPacket)
			log.Printf("Client Recv HotbarSync: %v", hb.Slots)
			c.Mutex.Lock()
			c.Hotbar = hb
			c.Mutex.Unlock()
		} else if packet.Type == network.PacketEquipmentSync {
			eq := packet.Data.(network.EquipmentSyncPacket)
			c.Mutex.Lock()
			c.Equipment = eq
			c.Mutex.Unlock()
		} else if packet.Type == network.PacketMapSync {
			m := packet.Data.(network.MapSyncPacket)
			c.Mutex.Lock()
			c.Map = m
			c.Mutex.Unlock()
		} else if packet.Type == network.PacketSpellbookSync {
			sb := packet.Data.(network.SpellbookSyncPacket)
			c.Mutex.Lock()
			c.UnlockedSpells = sb.UnlockedSpells
			// Also sync Cooldowns. Need to add Cooldowns field to Client first!
			c.Cooldowns = sb.Cooldowns
			c.Mutex.Unlock()
		}
	}
}

func (c *NetworkClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}
	c.Mutex.Lock()
	c.Inventory = network.InventorySyncPacket{}
	c.Hotbar = network.HotbarSyncPacket{}
	c.Equipment = network.EquipmentSyncPacket{}
	c.State = network.StateUpdatePacket{}
	c.Mutex.Unlock()
}

func (c *NetworkClient) SendInput(input components.InputComponent) {
	packet := network.Packet{
		Type: network.PacketInput,
		Data: network.InputPacket{Input: input},
	}
	// We handle errors loosely here for performance/simplicity
	_ = c.Encoder.Encode(packet)
}

func (c *NetworkClient) GetState() network.StateUpdatePacket {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.State
}

func (c *NetworkClient) GetInventory() network.InventorySyncPacket {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.Inventory
}

func (c *NetworkClient) GetHotbar() network.HotbarSyncPacket {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.Hotbar
}

func (c *NetworkClient) SendUpdateKeybindings(bindings map[string]int) {
	if c.Encoder != nil {
		packet := network.Packet{
			Type: network.PacketUpdateKeybindings,
			Data: network.UpdateKeybindingsPacket{Keybindings: bindings},
		}
		c.Encoder.Encode(packet)
	}
}

func (c *NetworkClient) SendUpdateDebugSettings(settings map[string]bool) {
	if c.Encoder != nil {
		packet := network.Packet{
			Type: network.PacketUpdateDebugSettings,
			Data: network.UpdateDebugSettingsPacket{Settings: settings},
		}
		c.Encoder.Encode(packet)
	}
}
func (c *NetworkClient) GetMap() network.MapSyncPacket {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.Map
}

func (c *NetworkClient) SendCastSpell(spellID string) {
	if c.Encoder != nil {
		packet := network.Packet{
			Type: network.PacketCastSpell,
			Data: network.CastSpellPacket{SpellID: spellID},
		}
		c.Encoder.Encode(packet)
	}
}
