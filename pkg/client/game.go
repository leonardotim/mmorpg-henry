package client

import (
	"fmt"
	"image/color"

	"henry/pkg/client/assets"
	"henry/pkg/client/systems"
	"henry/pkg/network"
	"henry/pkg/shared/config"
	protocol "henry/pkg/shared/network"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	ScreenWidth  = 800
	ScreenHeight = 600
)

type Game struct {
	Client *network.NetworkClient

	// Systems
	UISystem     *systems.UISystem
	InputSystem  *systems.InputSystem
	RenderSystem *systems.RenderSystem

	// State
	InGame   bool
	LoggedIn bool
	Username string

	// Inputs
	Keys map[string]ebiten.Key
}

func NewGame() *Game {
	protocol.RegisterGobTypes()
	assets.Load()
	g := &Game{
		Client: network.NewNetworkClient(),
		Keys:   make(map[string]ebiten.Key),
	}

	// Initialize default keys
	g.Keys["Up"] = ebiten.KeyW
	g.Keys["Down"] = ebiten.KeyS
	g.Keys["Left"] = ebiten.KeyA
	g.Keys["Right"] = ebiten.KeyD
	g.Keys["Hotbar1"] = ebiten.Key1
	g.Keys["Hotbar2"] = ebiten.Key2
	g.Keys["Hotbar3"] = ebiten.Key3
	g.Keys["Hotbar4"] = ebiten.Key4
	g.Keys["Hotbar5"] = ebiten.Key5
	g.Keys["Hotbar6"] = ebiten.Key6
	g.Keys["Hotbar7"] = ebiten.Key7
	g.Keys["Hotbar8"] = ebiten.Key8
	g.Keys["Hotbar9"] = ebiten.Key9
	g.Keys["Hotbar0"] = ebiten.Key0
	g.Keys["Inventory"] = ebiten.KeyI
	g.Keys["Spells"] = ebiten.KeyM
	g.Keys["Equipment"] = ebiten.KeyE
	g.Keys["Menu"] = ebiten.KeyEscape
	g.Keys["Bind"] = ebiten.KeyB
	g.Keys[config.ActionRun] = ebiten.KeyShift
	// MouseButtonLeft is handled separately as it's not ebiten.Key

	// Initialize Systems
	// Initialize Systems
	g.UISystem = systems.NewUISystem(g.Client, g.Keys)
	g.UISystem.Init()

	g.UISystem.RegisterDisconnectCallback(func() {
		g.LoggedIn = false
		g.Client.Close()
		g.UISystem.ResetUI()
		g.UISystem.SpellsWidget.UnlockedSpells = make(map[string]bool)
	})

	g.UISystem.RegisterLoginCallback(func(user, pass string, isSignup bool) {
		var keys map[string]int
		var err error

		if isSignup {
			err = g.Client.Signup("127.0.0.1:8080", user, pass)
			if err != nil {
				fmt.Printf("Signup Error: %v\n", err)
				return
			}
			fmt.Println("Signup Success! Please Login.")
		} else {
			var debugSettings map[string]bool
			var openMenus map[string]bool
			var isRunning bool // Declare isRunning
			keys, debugSettings, openMenus, isRunning, err = g.Client.Connect("127.0.0.1:8080", user, pass)
			if err != nil {
				fmt.Printf("Login Error: %v\n", err)
				return
			}
			g.LoggedIn = true
			g.Username = user
			g.UISystem.HideLogin()
			g.UISystem.ApplyOpenMenus(openMenus)
			g.InputSystem.SetRunning(isRunning) // Pass the persisted state

			// Apply Keys
			if keys != nil {
				for k, v := range keys {
					if v != 0 {
						g.Keys[k] = ebiten.Key(v)
					}
				}
			}

			// Apply Debug Settings
			if debugSettings != nil {
				g.UISystem.DebugFlags.ShowFPS = debugSettings["ShowFPS"]
				g.UISystem.DebugFlags.ShowInfo = debugSettings["ShowInfo"]
				g.UISystem.DebugFlags.ShowLogs = debugSettings["ShowLogs"]
				g.UISystem.DebugFlags.ShowLogs = debugSettings["ShowLogs"]
			}

			// Sync Unlocked Spells
			if g.Client.UnlockedSpells != nil {
				// Reset first?
				g.UISystem.SpellsWidget.UnlockedSpells = make(map[string]bool)
				for _, spellID := range g.Client.UnlockedSpells {
					g.UISystem.SpellsWidget.UnlockedSpells[spellID] = true
				}
			}
		}
	})

	g.InputSystem = systems.NewInputSystem(g.Client, g.UISystem, g.Keys)
	g.RenderSystem = systems.NewRenderSystem(g.Client, g.UISystem)

	return g
}

func (g *Game) Update() error {
	// Update Network (Reading packets is in goroutine, but we might need to handle channel if we had one.
	// Current impl just updates state in mutex.)

	g.UISystem.Update()

	if !g.LoggedIn {
		return nil
	}

	g.HandleInput()

	return nil
}

func (g *Game) HandleInput() {
	// Global Toggles via System
	g.InputSystem.HandleGlobalKeys()

	if g.UISystem.IsInputCaptured() {
		return
	}

	// Gameplay Input via System
	g.InputSystem.Update()
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 20, G: 60, B: 20, A: 255}) // Dark green background

	if !g.LoggedIn {
		g.UISystem.Draw(screen)
		return
	}

	g.RenderSystem.Draw(screen)

	// UI is drawn by RenderSystem
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}
