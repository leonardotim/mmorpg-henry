package main

import (
	"log"

	"henry/pkg/client"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	game := client.NewGame()

	ebiten.SetWindowSize(client.ScreenWidth, client.ScreenHeight)
	ebiten.SetWindowTitle("Henry MMORPG (WASM Ready)")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
