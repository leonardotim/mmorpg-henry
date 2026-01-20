package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	fmt.Printf("A: %d\n", ebiten.KeyA)
	fmt.Printf("D: %d\n", ebiten.KeyD)
	fmt.Printf("S: %d\n", ebiten.KeyS)
	fmt.Printf("W: %d\n", ebiten.KeyW)
	fmt.Printf("Left: %d\n", ebiten.KeyLeft)
	fmt.Printf("Right: %d\n", ebiten.KeyRight)
	fmt.Printf("Up: %d\n", ebiten.KeyUp)
	fmt.Printf("Down: %d\n", ebiten.KeyDown)

	// Check specific value
	fmt.Printf("Key(29): %s\n", ebiten.Key(29).String())
}
