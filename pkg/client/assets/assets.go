package assets

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed images/*.png
var assetsFS embed.FS

var images = make(map[string]*ebiten.Image)

func Load() {
	// Load Fireball
	loadHasIcon("fireball", "images/fireball.png")
	log.Println("Assets loaded.")
}

func loadHasIcon(name, path string) {
	data, err := assetsFS.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read asset %s: %v", path, err)
		return
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to decode asset %s: %v", path, err)
		return
	}

	images[name] = ebiten.NewImageFromImage(img)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	log.Printf("Loaded asset %s (%dx%d)", path, w, h)
}

func GetImage(name string) *ebiten.Image {
	return images[name]
}
