package assets

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	_ "image/png"
	"log"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed images/*.png characters projectiles/*.png
var assetsFS embed.FS

var images = make(map[string]*ebiten.Image)

// Map[CharacterName] -> AnimationName -> Direction -> []Frames
var characterAnimations = make(map[string]map[string]map[string][]*ebiten.Image)

// Standard rotation-only map fallback (optional if we migrate fully to animations)
// Map[CharacterName][Direction] -> Image
var characterSprites = make(map[string]map[string]*ebiten.Image)

type CharacterMetadata struct {
	Character struct {
		Name string `json:"name"`
	} `json:"character"`
	Frames struct {
		Rotations  map[string]string              `json:"rotations"`
		Animations map[string]map[string][]string `json:"animations"` // anim -> dir -> []files
	} `json:"frames"`
}

func Load() {
	// Load Projectiles
	loadHasIcon("fireball", "images/fireball.png")
	loadHasIcon("arrow", "projectiles/arrow.png")

	// Load Player Character
	if err := LoadCharacter("player", "characters/player/metadata.json"); err != nil {
		log.Printf("Failed to load player character: %v", err)
	}

	// Load Guard Character
	if err := LoadCharacter("guard", "characters/guard/metadata.json"); err != nil {
		log.Printf("Failed to load guard character: %v", err)
	}

	log.Println("Assets loaded.")
}

func LoadCharacter(charName, metadataPath string) error {
	data, err := assetsFS.ReadFile(metadataPath)
	if err != nil {
		return err
	}

	var meta CharacterMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	// Initialize Maps
	characterSprites[charName] = make(map[string]*ebiten.Image)
	if characterAnimations[charName] == nil {
		characterAnimations[charName] = make(map[string]map[string][]*ebiten.Image)
	}

	baseDir := filepath.Dir(metadataPath)

	// 1. Load Static Rotations (Fallback)
	for dir, relPath := range meta.Frames.Rotations {
		fullPath := filepath.Join(baseDir, relPath)
		img, err := loadImage(fullPath)
		if err != nil {
			log.Printf("Failed to load static rotation %s %s: %v", charName, dir, err)
			continue
		}
		characterSprites[charName][dir] = img
	}

	// 2. Load Animations
	for animName, directions := range meta.Frames.Animations {
		characterAnimations[charName][animName] = make(map[string][]*ebiten.Image)

		for dir, filePaths := range directions {
			var frames []*ebiten.Image
			for _, relPath := range filePaths {
				fullPath := filepath.Join(baseDir, relPath)
				img, err := loadImage(fullPath)
				if err != nil {
					log.Printf("Failed to load animation frame %s %s %s: %v", charName, animName, relPath, err)
					continue
				}
				frames = append(frames, img)
			}
			characterAnimations[charName][animName][dir] = frames
			log.Printf("Loaded animation %s for %s (%s): %d frames", animName, charName, dir, len(frames))
		}
	}

	return nil
}

func loadImage(path string) (*ebiten.Image, error) {
	imgData, err := assetsFS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, err
	}
	return ebiten.NewImageFromImage(img), nil
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

func GetCharacterSprite(name, direction string) *ebiten.Image {
	if sprites, ok := characterSprites[name]; ok {
		return sprites[direction]
	}
	return nil
}

func GetCharacterFrame(charName, animName, direction string, frameIndex int) *ebiten.Image {
	if charName == "" || animName == "" || direction == "" {
		return nil
	}

	if anims, ok := characterAnimations[charName]; ok {
		if dirs, ok := anims[animName]; ok {
			if frames, ok := dirs[direction]; ok && len(frames) > 0 {
				return frames[frameIndex%len(frames)] // Loop safely
			}
		}
	}
	// Fallback to static sprite if animation missing
	return GetCharacterSprite(charName, direction)
}
