package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Element is the base interface for all UI widgets
type Element interface {
	Update() (bool, error)
	Draw(screen *ebiten.Image)
	HandleInput(x, y int) bool // Returns true if input was consumed
	SetPosition(x, y float64)
	GetPosition() (float64, float64)
	GetSize() (float64, float64)
	IsVisible() bool
	SetVisible(visible bool)
}

// BaseElement holds common properties
type BaseElement struct {
	X, Y          float64
	Width, Height float64
	Visible       bool
	Color         color.Color
}

func (b *BaseElement) SetPosition(x, y float64) {
	b.X = x
	b.Y = y
}

func (b *BaseElement) GetPosition() (float64, float64) {
	return b.X, b.Y
}

func (b *BaseElement) GetSize() (float64, float64) {
	return b.Width, b.Height
}

func (b *BaseElement) IsVisible() bool {
	return b.Visible
}

func (b *BaseElement) SetVisible(visible bool) {
	b.Visible = visible
}

// Button Styles
type ButtonStyle int

const (
	ButtonStylePrimary ButtonStyle = iota
	ButtonStyleSecondary
	ButtonStyleDestructive
)

// Button Widget
type Button struct {
	BaseElement
	Text      string
	OnClick   func()
	IsHovered bool
	Style     ButtonStyle
}

func NewButton(x, y, w, h float64, text string, onClick func()) *Button {
	return &Button{
		BaseElement: BaseElement{X: x, Y: y, Width: w, Height: h, Visible: true},
		Text:        text,
		OnClick:     onClick,
		Style:       ButtonStylePrimary,
	}
}

func NewSecondaryButton(x, y, w, h float64, text string, onClick func()) *Button {
	return &Button{
		BaseElement: BaseElement{X: x, Y: y, Width: w, Height: h, Visible: true},
		Text:        text,
		OnClick:     onClick,
		Style:       ButtonStyleSecondary,
	}
}

// Button Widget Update
func (b *Button) Update() (bool, error) {
	if !b.Visible {
		return false, nil
	}

	mx, my := ebiten.CursorPosition()
	b.IsHovered = mx >= int(b.X) && mx <= int(b.X+b.Width) && my >= int(b.Y) && my <= int(b.Y+b.Height)

	if b.IsHovered && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if b.OnClick != nil {
			b.OnClick()
			return true, nil // Consumed
		}
	}
	return false, nil
}

func (b *Button) Draw(screen *ebiten.Image) {
	if !b.Visible {
		return
	}

	var bgColor color.Color
	var borderColor color.Color

	switch b.Style {
	case ButtonStylePrimary:
		if b.IsHovered {
			bgColor = color.RGBA{100, 100, 200, 255} // Brighter Blue
		} else {
			bgColor = color.RGBA{60, 60, 180, 255} // Blue
		}
		borderColor = color.RGBA{200, 200, 255, 255}
	case ButtonStyleSecondary:
		if b.IsHovered {
			bgColor = color.RGBA{80, 80, 80, 255} // Lighter Gray
		} else {
			bgColor = color.RGBA{40, 40, 40, 255} // Dark Gray
		}
		borderColor = color.RGBA{100, 100, 100, 255}
	case ButtonStyleDestructive:
		if b.IsHovered {
			bgColor = color.RGBA{200, 80, 80, 255}
		} else {
			bgColor = color.RGBA{180, 40, 40, 255}
		}
		borderColor = color.RGBA{255, 100, 100, 255}
	}

	// Draw Background
	ebitenutil.DrawRect(screen, b.X, b.Y, b.Width, b.Height, bgColor)

	// Draw Border
	ebitenutil.DrawLine(screen, b.X, b.Y, b.X+b.Width, b.Y, borderColor)
	ebitenutil.DrawLine(screen, b.X, b.Y, b.X, b.Y+b.Height, borderColor)
	ebitenutil.DrawLine(screen, b.X+b.Width, b.Y, b.X+b.Width, b.Y+b.Height, borderColor)
	ebitenutil.DrawLine(screen, b.X, b.Y+b.Height, b.X+b.Width, b.Y+b.Height, borderColor)

	// Draw Text
	textWidth := len(b.Text) * 7
	textX := int(b.X) + (int(b.Width)-textWidth)/2
	if textX < int(b.X)+5 {
		textX = int(b.X) + 5
	}
	ebitenutil.DebugPrintAt(screen, b.Text, textX, int(b.Y+b.Height/2-8))
}

func (b *Button) HandleInput(x, y int) bool {
	if !b.Visible {
		return false
	}
	return x >= int(b.X) && x <= int(b.X+b.Width) && y >= int(b.Y) && y <= int(b.Y+b.Height)
}

// Manager handles the UI stack
type Manager struct {
	Elements []Element
}

func NewManager() *Manager {
	return &Manager{
		Elements: make([]Element, 0),
	}
}

func (m *Manager) AddElement(e Element) {
	m.Elements = append(m.Elements, e)
}

// Manager Update
func (m *Manager) Update() error {
	// Iterate backwards so top-most elements (added last) handle input first.
	// We check if input was consumed and break if so.
	for i := len(m.Elements) - 1; i >= 0; i-- {
		consumed, err := m.Elements[i].Update()
		if err != nil {
			return err
		}
		if consumed {
			// Stop processing any other elements this frame
			break
		}
	}
	return nil
}

func (m *Manager) Draw(screen *ebiten.Image) {
	for _, e := range m.Elements {
		e.Draw(screen)
	}
}

// Helper to check if mouse is over ANY UI element
func (m *Manager) IsMouseOverUI() bool {
	mx, my := ebiten.CursorPosition()
	for _, e := range m.Elements {
		if e.IsVisible() && e.HandleInput(mx, my) {
			return true
		}
	}
	return false
}
