package ui

import (
	"henry/pkg/client/assets"
	"henry/pkg/shared/components"
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Label Widget
type Label struct {
	BaseElement
	Text string
}

func NewLabel(x, y float64, text string) *Label {
	return &Label{
		BaseElement: BaseElement{X: x, Y: y, Width: 0, Height: 0, Visible: true, Color: color.White},
		Text:        text,
	}
}

// Label Update
func (l *Label) Update() (bool, error) {
	return false, nil
}

func (l *Label) Draw(screen *ebiten.Image) {
	if !l.Visible {
		return
	}
	ebitenutil.DebugPrintAt(screen, l.Text, int(l.X), int(l.Y))
}

func (l *Label) HandleInput(x, y int) bool {
	return false // Labels don't consume input usually
}

// Window Container
type WindowChild struct {
	Element Element
	RelX    float64
	RelY    float64
	Fixed   bool
}

type Window struct {
	BaseElement
	Title                    string
	Children                 []WindowChild
	Draggable                bool
	IsDragging               bool
	DragOffsetX, DragOffsetY float64
	ScrollY                  float64
	ContentHeight            float64
	FooterHeight             float64
	ShowScrollbar            bool
}

func NewWindow(x, y, w, h float64, title string) *Window {
	return &Window{
		BaseElement:   BaseElement{X: x, Y: y, Width: w, Height: h, Visible: false, Color: color.RGBA{50, 50, 50, 240}},
		Title:         title,
		Children:      make([]WindowChild, 0),
		Draggable:     false,
		ShowScrollbar: true,
	}
}

func (w *Window) SetBackButton(onClick func()) {
	w.FooterHeight = 40
	btn := NewSecondaryButton(10, w.Height-55, w.Width-20, 30, "Back", onClick)
	w.AddChildOption(btn, true)
}

func (w *Window) AddChild(e Element) {
	w.AddChildOption(e, false)
}

func (w *Window) AddChildOption(e Element, fixed bool) {
	rx, ry := e.GetPosition()
	w.Children = append(w.Children, WindowChild{Element: e, RelX: rx, RelY: ry, Fixed: fixed})

	if !fixed {
		// Update ContentHeight only for scrollable items
		_, h := e.GetSize()
		childBottom := ry + h
		// Add padding to ensure last item can scroll up fully with room to spare
		if childBottom+10 > w.ContentHeight {
			w.ContentHeight = childBottom + 10
		}
		// Initial Pos update for scrollable
		e.SetPosition(w.X+rx, w.Y+ry+20-w.ScrollY)
	} else {
		// Initial Pos update for fixed
		e.SetPosition(w.X+rx, w.Y+ry+20)
	}
}

// Window Update
func (w *Window) Update() (bool, error) {
	if !w.Visible {
		return false, nil
	}

	consumed := false
	mx, my := ebiten.CursorPosition()

	// Handle Dragging
	if w.Draggable && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= int(w.X) && mx <= int(w.X+w.Width) && my >= int(w.Y) && my <= int(w.Y+20) {
			w.IsDragging = true
			w.DragOffsetX = float64(mx) - w.X
			w.DragOffsetY = float64(my) - w.Y
			consumed = true
		}
	}

	if w.IsDragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			w.X = float64(mx) - w.DragOffsetX
			w.Y = float64(my) - w.DragOffsetY
			consumed = true
		} else {
			w.IsDragging = false
		}
	}

	// Calculate Viewable Area
	viewHeight := w.Height - 20 - w.FooterHeight // Height minus Title and Footer

	// Handle Scrolling
	if w.ContentHeight > viewHeight {
		_, wy := ebiten.Wheel()
		if wy != 0 {
			// Check hover
			if mx >= int(w.X) && mx <= int(w.X+w.Width) && my >= int(w.Y) && my <= int(w.Y+w.Height) {
				w.ScrollY -= wy * 5 // Reduced from 20 to 5 for smoother scrolling
				// Clamp
				maxScroll := w.ContentHeight - viewHeight
				if w.ScrollY < 0 {
					w.ScrollY = 0
				}
				if w.ScrollY > maxScroll {
					w.ScrollY = maxScroll
				}
				consumed = true
			}
		}
	}

	// Update children positions and visibility
	for i := len(w.Children) - 1; i >= 0; i-- {
		child := &w.Children[i]

		if child.Fixed {
			// Fixed elements (like Back button footer)
			child.Element.SetPosition(w.X+child.RelX, w.Y+20+child.RelY)
			child.Element.SetVisible(true)
		} else {
			// Scrollable elements
			absY := w.Y + 20 + child.RelY - w.ScrollY
			child.Element.SetPosition(w.X+child.RelX, absY)

			// Visibility culling (Clipping)
			_, ch := child.Element.GetSize()

			// Clip Top: Below Title (w.Y + 20)
			// Clip Bottom: Above Footer (w.Y + w.Height - w.FooterHeight)
			clipBottom := w.Y + w.Height - w.FooterHeight

			if absY < w.Y+20 || absY+ch > clipBottom {
				child.Element.SetVisible(false)
			} else {
				child.Element.SetVisible(true)
			}
		}

		childConsumed, err := child.Element.Update()
		if err != nil {
			return consumed, err
		}
		if childConsumed {
			consumed = true
		}
	}
	return consumed, nil
}

func (w *Window) Draw(screen *ebiten.Image) {
	if !w.Visible {
		return
	}

	// Draw Window Body
	ebitenutil.DrawRect(screen, w.X, w.Y, w.Width, w.Height, w.Color)

	// Draw Children
	for _, child := range w.Children {
		child.Element.Draw(screen)
	}

	// Draw Title Bar (Overlay to hide scrolled-up items)
	ebitenutil.DrawRect(screen, w.X, w.Y, w.Width, 20, color.RGBA{80, 80, 80, 255})
	ebitenutil.DebugPrintAt(screen, w.Title, int(w.X+5), int(w.Y+2))

	// Draw Bottom Overlay? (To hide scrolled-down items peeking)
	// Optional, but clean.
	// Actually, drawing the border on top works well enough.

	// Draw Border
	ebitenutil.DrawLine(screen, w.X, w.Y, w.X+w.Width, w.Y, color.White)
	ebitenutil.DrawLine(screen, w.X, w.Y, w.X, w.Y+w.Height, color.White)
	ebitenutil.DrawLine(screen, w.X+w.Width, w.Y, w.X+w.Width, w.Y+w.Height, color.White)
	ebitenutil.DrawLine(screen, w.X, w.Y+w.Height, w.X+w.Width, w.Y+w.Height, color.White)

	// Draw Scrollbar?
	if w.ShowScrollbar {
		viewHeight := w.Height - 20 - w.FooterHeight
		if w.ContentHeight > viewHeight {
			maxScroll := w.ContentHeight - viewHeight
			if maxScroll > 0 {
				scrollPct := w.ScrollY / maxScroll
				barHeight := viewHeight * (viewHeight / w.ContentHeight)
				if barHeight < 20 {
					barHeight = 20
				}

				barSpace := viewHeight - barHeight
				barY := w.Y + 20 + scrollPct*barSpace

				ebitenutil.DrawRect(screen, w.X+w.Width-5, barY, 5, barHeight, color.RGBA{150, 150, 150, 255})
			}
		}
	}
}

func (w *Window) HandleInput(x, y int) bool {
	if !w.Visible {
		return false
	}
	// Check children first (buttons on top)
	for _, child := range w.Children {
		if child.Element.HandleInput(x, y) {
			return true
		}
	}

	// Then check window itself
	return x >= int(w.X) && x <= int(w.X+w.Width) && y >= int(w.Y) && y <= int(w.Y+w.Height)
}

// Inventory Widget
type InventoryWidget struct {
	BaseElement
	Slots    []string // Item IDs
	SlotSize float64
	Cols     int

	// Drag & Drop State
	DraggingIndex int // -1 if none
	DragItem      string
	DragStartX    float64
	DragStartY    float64
	DragCurrX     float64
	DragCurrY     float64

	// Interaction Callbacks
	// Interaction Callbacks
	OnItemDrop       func(fromIndex, toIndex int)
	OnSlotRightClick func(index int, x, y int)

	// Display Config
	SlotOffset  int
	ShowHotkeys bool
	HiddenIndex int // Slot index to hide (e.g. being dragged)
}

func NewInventoryWidget(x, y float64, cols, rows int, slotSize float64) *InventoryWidget {
	w := float64(cols) * slotSize
	h := float64(rows) * slotSize
	return &InventoryWidget{
		BaseElement: BaseElement{X: x, Y: y, Width: w, Height: h, Visible: true},
		Slots:       make([]string, cols*rows),
		SlotSize:    slotSize,
		Cols:        cols,
		HiddenIndex: -1,
	}
}

// InventoryWidget Update
func (iw *InventoryWidget) Update() (bool, error) {
	if !iw.Visible {
		return false, nil
	}

	mx, my := ebiten.CursorPosition()
	consumed := false

	// Handle Drag Start / Click
	// We rely on parent system to handle actual drag state logic.
	// But we detect the initial click here?
	// Or even better: UISystem checks IsHovered and MouseButtonJustPressed.

	// Let's keep Right Click here as it's simple
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if iw.IsHovered(mx, my) {
			index := iw.GetSlotAt(mx, my)
			if index != -1 && iw.Slots[index] != "" {
				if iw.OnSlotRightClick != nil {
					iw.OnSlotRightClick(index+iw.SlotOffset, mx, my)
				}
				consumed = true
			}
		}
	}

	return consumed, nil
}

func (iw *InventoryWidget) GetSlotAt(mx, my int) int {
	if !iw.IsHovered(mx, my) {
		return -1
	}
	rx := float64(mx) - iw.X
	ry := float64(my) - iw.Y
	col := int(rx / iw.SlotSize)
	row := int(ry / iw.SlotSize)
	index := row*iw.Cols + col
	if index >= 0 && index < len(iw.Slots) {
		return index
	}
	return -1
}

func (iw *InventoryWidget) getSlotAt(mx, my int) int {
	if !iw.IsHovered(mx, my) {
		return -1
	}
	rx := float64(mx) - iw.X
	ry := float64(my) - iw.Y
	col := int(rx / iw.SlotSize)
	row := int(ry / iw.SlotSize)
	index := row*iw.Cols + col
	if index >= 0 && index < len(iw.Slots) {
		return index
	}
	return -1
}

// --- Context Menu ---

type MenuOption struct {
	Text   string
	Action func()
}

type ContextMenu struct {
	BaseElement
	Buttons []*Button
}

func NewContextMenu() *ContextMenu {
	return &ContextMenu{
		BaseElement: BaseElement{Visible: false},
		Buttons:     make([]*Button, 0),
	}
}

func (cm *ContextMenu) Show(x, y float64, options []MenuOption, minX, minY, maxX, maxY float64) {
	cm.Width = 100 // Fixed width for now
	cm.Buttons = make([]*Button, 0)

	offsetY := 0.0
	for _, opt := range options {
		// Capture action for closure
		action := opt.Action
		btn := NewButton(0, 0, 100, 25, opt.Text, func() {
			if action != nil {
				action()
			}
			cm.Visible = false // Auto-close
		})
		btn.Style = ButtonStyleSecondary // Darker style
		cm.Buttons = append(cm.Buttons, btn)
		offsetY += 25
	}
	cm.Height = offsetY

	// Parent Bounds Clamp
	if x+cm.Width > maxX {
		x = maxX - cm.Width - 5
	}
	if x < minX {
		x = minX + 5
	}

	if y+cm.Height > maxY {
		y = maxY - cm.Height - 5
	}
	if y < minY {
		y = minY + 5
	}

	cm.X = x
	cm.Y = y

	// Update Buttons absolute positions
	for i, btn := range cm.Buttons {
		btn.X = x
		btn.Y = y + float64(i)*25
	}

	cm.Visible = true
}

func (cm *ContextMenu) Hide() {
	cm.Visible = false
}

func (cm *ContextMenu) Update() (bool, error) {
	if !cm.Visible {
		return false, nil
	}

	consumed := false

	// Check click outside to close
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		// If click is NOT inside menu, close it
		// Note: We need to check if click is inside any button.
		// Detailed check:
		isInside := float64(mx) >= cm.X && float64(mx) <= cm.X+cm.Width &&
			float64(my) >= cm.Y && float64(my) <= cm.Y+cm.Height

		if !isInside {
			cm.Visible = false
			// Don't consume here, let others handle it?
			// Actually, if we close menu, we probably consumed "closing the menu".
			// But if we clicked another button, we want that to work?
			// Standard behavior: click outside consumes the "close" event but doesn't trigger anything else?
			// Or pass through. Let's pass through for now or consume to be safe.
			// Let's consume so we don't accidentally click something underneath immediately.
			consumed = true
		}
	}

	// Update Buttons
	for _, btn := range cm.Buttons {
		if c, _ := btn.Update(); c {
			consumed = true
		}
	}

	return consumed, nil
}

func (cm *ContextMenu) Draw(screen *ebiten.Image) {
	if !cm.Visible {
		return
	}
	// Background
	ebitenutil.DrawRect(screen, cm.X, cm.Y, cm.Width, cm.Height, color.RGBA{40, 40, 40, 255})
	// Border
	ebitenutil.DrawLine(screen, cm.X, cm.Y, cm.X+cm.Width, cm.Y, color.Gray{150})
	ebitenutil.DrawLine(screen, cm.X, cm.Y, cm.X, cm.Y+cm.Height, color.Gray{150})
	ebitenutil.DrawLine(screen, cm.X+cm.Width, cm.Y, cm.X+cm.Width, cm.Y+cm.Height, color.Gray{150})
	ebitenutil.DrawLine(screen, cm.X, cm.Y+cm.Height, cm.X+cm.Width, cm.Y+cm.Height, color.Gray{150})

	for _, btn := range cm.Buttons {
		btn.Draw(screen)
	}
}

func (cm *ContextMenu) HandleInput(x, y int) bool {
	if !cm.Visible {
		return false
	}
	// Check buttons
	for _, btn := range cm.Buttons {
		if btn.HandleInput(x, y) {
			return true
		}
	}
	return false
}

func (iw *InventoryWidget) Draw(screen *ebiten.Image) {
	if !iw.Visible {
		return
	}

	for i, itemID := range iw.Slots {
		col := i % iw.Cols
		row := i / iw.Cols

		sx := iw.X + float64(col)*iw.SlotSize
		sy := iw.Y + float64(row)*iw.SlotSize

		// Draw Slot Background
		c := color.RGBA{60, 60, 60, 255}
		ebitenutil.DrawRect(screen, sx+1, sy+1, iw.SlotSize-2, iw.SlotSize-2, c)

		// Draw Item
		if itemID != "" && (i != iw.HiddenIndex) {
			// Look for Icon
			if img := assets.GetImage(itemID); img != nil {
				opts := &ebiten.DrawImageOptions{}
				w, h := img.Size()
				scaleX := (iw.SlotSize - 4) / float64(w)
				scaleY := (iw.SlotSize - 4) / float64(h)
				opts.GeoM.Scale(scaleX, scaleY)
				opts.GeoM.Translate(sx+2, sy+2)
				screen.DrawImage(img, opts)
			} else {
				// Draw Item Color/Icon Fallback
				ebitenutil.DrawRect(screen, sx+5, sy+5, iw.SlotSize-10, iw.SlotSize-10, color.RGBA{200, 100, 100, 255})
				ebitenutil.DebugPrintAt(screen, itemID[:1], int(sx+10), int(sy+10))
			}
		}

		// Draw Hotkey Number
		if iw.ShowHotkeys {
			num := (i + 1) % 10
			label := string(rune('0' + num))
			ebitenutil.DebugPrintAt(screen, label, int(sx+iw.SlotSize-12), int(sy+2))
		}

		// Border
		ebitenutil.DrawLine(screen, sx, sy, sx+iw.SlotSize, sy, color.Gray{100})
		ebitenutil.DrawLine(screen, sx, sy, sx, sy+iw.SlotSize, color.Gray{100})
	}
}

func (iw *InventoryWidget) HandleInput(x, y int) bool {
	return iw.IsHovered(x, y)
}

func (iw *InventoryWidget) IsHovered(x, y int) bool {
	return float64(x) >= iw.X && float64(x) <= iw.X+iw.Width && float64(y) >= iw.Y && float64(y) <= iw.Y+iw.Height
}

// TextInput Widget
type TextInput struct {
	BaseElement
	Text        string
	Placeholder string
	Focused     bool
	Counter     int
	IsPassword  bool
}

func NewTextInput(x, y, w, h float64, placeholder string) *TextInput {
	return &TextInput{
		BaseElement: BaseElement{X: x, Y: y, Width: w, Height: h, Visible: true},
		Placeholder: placeholder,
	}
}

func (t *TextInput) Update() (bool, error) {
	if !t.Visible {
		return false, nil
	}

	t.Counter++

	if t.Focused {
		// Handle Text Input
		runes := ebiten.InputChars()
		t.Text = string(append([]rune(t.Text), runes...))

		// Handle Backspace
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if len(t.Text) > 0 {
				t.Text = t.Text[:len(t.Text)-1]
			}
		}
	}

	// Handle Focus Click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float64(mx) >= t.X && float64(mx) <= t.X+t.Width && float64(my) >= t.Y && float64(my) <= t.Y+t.Height {
			t.Focused = true
			return true, nil // Consumed click on input
		} else {
			t.Focused = false
		}
	}

	return false, nil
}

func (t *TextInput) Draw(screen *ebiten.Image) {
	if !t.Visible {
		return
	}

	// Draw Box
	c := color.RGBA{30, 30, 30, 255}
	if t.Focused {
		c = color.RGBA{50, 50, 50, 255}
	}
	ebitenutil.DrawRect(screen, t.X, t.Y, t.Width, t.Height, c)

	// Draw Border
	borderColor := color.RGBA{255, 255, 255, 255}
	if t.Focused {
		borderColor = color.RGBA{255, 255, 255, 255}
	}
	ebitenutil.DrawLine(screen, t.X, t.Y, t.X+t.Width, t.Y, borderColor)
	ebitenutil.DrawLine(screen, t.X, t.Y+t.Height, t.X+t.Width, t.Y+t.Height, borderColor)
	ebitenutil.DrawLine(screen, t.X, t.Y, t.X, t.Y+t.Height, borderColor)
	ebitenutil.DrawLine(screen, t.X+t.Width, t.Y, t.X+t.Width, t.Y+t.Height, borderColor)

	// Draw Text
	display := t.Text
	if t.IsPassword {
		display = strings.Repeat("*", len(t.Text))
	}

	if display == "" && !t.Focused {
		display = t.Placeholder
	}

	// Cursor
	if t.Focused && (t.Counter/30)%2 == 0 {
		display += "|"
	}

	ebitenutil.DebugPrintAt(screen, display, int(t.X+5), int(t.Y+10))
}

func (t *TextInput) HandleInput(x, y int) bool {
	// If clicked, we might consume it.
	// But focus logic is in Update for now.
	// Return true if hovered to block other inputs?
	return float64(x) >= t.X && float64(x) <= t.X+t.Width && float64(y) >= t.Y && float64(y) <= t.Y+t.Height
}

// SpellsWidget
type SpellsWidget struct {
	BaseElement
	Slots    []string // Spell IDs
	ColCount int
	SlotSize float64

	// Logic
	UnlockedSpells map[string]bool
	Cooldowns      map[string]float64
	ActiveSpellID  string

	// Tooltip State
	HoveredSpellID     string
	TooltipX, TooltipY float64

	// Interactions
	OnSpellClick func(spellID string, isRightClick bool)
}

func NewSpellsWidget(x, y float64, cols, rows int, slotSize float64) *SpellsWidget {
	w := float64(cols) * slotSize
	h := float64(rows) * slotSize
	return &SpellsWidget{
		BaseElement:    BaseElement{X: x, Y: y, Width: w, Height: h, Visible: true},
		Slots:          make([]string, cols*rows),
		ColCount:       cols,
		SlotSize:       slotSize,
		UnlockedSpells: make(map[string]bool),
		Cooldowns:      make(map[string]float64),
	}
}

func (sw *SpellsWidget) Update() (bool, error) {
	if !sw.Visible {
		return false, nil
	}

	mx, my := ebiten.CursorPosition()
	sw.HoveredSpellID = "" // Reset
	consumed := false

	if sw.IsHovered(mx, my) {
		// Calculate Hovered Slot
		col := int((float64(mx) - sw.X) / sw.SlotSize)
		row := int((float64(my) - sw.Y) / sw.SlotSize)
		index := row*sw.ColCount + col

		if index >= 0 && index < len(sw.Slots) && sw.Slots[index] != "" {
			sw.HoveredSpellID = sw.Slots[index]
			sw.TooltipX = float64(mx) + 15
			sw.TooltipY = float64(my) + 15

			// Handle Clicks
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if sw.OnSpellClick != nil {
					sw.OnSpellClick(sw.HoveredSpellID, false)
				}
				consumed = true
			}
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
				if sw.OnSpellClick != nil {
					sw.OnSpellClick(sw.HoveredSpellID, true)
				}
				consumed = true
			}
		}
	}
	return consumed, nil
}

func (sw *SpellsWidget) Draw(screen *ebiten.Image) {
	if !sw.Visible {
		return
	}

	for i, spellID := range sw.Slots {
		col := i % sw.ColCount
		row := i / sw.ColCount
		sx := sw.X + float64(col)*sw.SlotSize
		sy := sw.Y + float64(row)*sw.SlotSize

		// Draw Slot Background (Consistent with Inventory)
		ebitenutil.DrawRect(screen, sx+1, sy+1, sw.SlotSize-2, sw.SlotSize-2, color.RGBA{60, 60, 60, 255})

		// Draw Border
		ebitenutil.DrawLine(screen, sx, sy, sx+sw.SlotSize, sy, color.Gray{100})
		ebitenutil.DrawLine(screen, sx, sy, sx, sy+sw.SlotSize, color.Gray{100})

		// Skip drawing content if empty
		if spellID == "" {
			continue
		}

		// Content Logic
		unlocked := sw.UnlockedSpells[spellID]
		spellDef, exists := components.SpellRegistry[spellID]
		if !exists {
			continue
		}

		orbSize := sw.SlotSize - 10
		ox := sx + 5
		oy := sy + 5

		c := spellDef.Color
		if !unlocked {
			c = color.RGBA{100, 100, 100, 255} // Grey
		}

		// Orb or Icon
		if img := assets.GetImage(spellDef.Icon); img != nil {
			opts := &ebiten.DrawImageOptions{}
			iw, ih := img.Size()
			scaleX := orbSize / float64(iw)
			scaleY := orbSize / float64(ih)
			opts.GeoM.Scale(scaleX, scaleY)
			opts.GeoM.Translate(ox, oy)
			if !unlocked {
				opts.ColorM.Scale(0.5, 0.5, 0.5, 1)
			}
			screen.DrawImage(img, opts)
		} else {
			ebitenutil.DrawRect(screen, ox, oy, orbSize, orbSize, c)
		}

		// Active Selection Border (Turquoise) - Overrides standard border if active
		if sw.ActiveSpellID == spellID {
			borderC := color.RGBA{64, 224, 208, 255} // Turquoise
			for w := 0.0; w < 3.0; w++ {
				ebitenutil.DrawLine(screen, sx, sy+w, sx+sw.SlotSize, sy+w, borderC)
				ebitenutil.DrawLine(screen, sx, sy+sw.SlotSize-w, sx+sw.SlotSize, sy+sw.SlotSize-w, borderC)
				ebitenutil.DrawLine(screen, sx+w, sy, sx+w, sy+sw.SlotSize, borderC)
				ebitenutil.DrawLine(screen, sx+sw.SlotSize-w, sy, sx+sw.SlotSize-w, sy+sw.SlotSize, borderC)
			}
		}

		// Cooldown Overlay
		if lastCast, ok := sw.Cooldowns[spellID]; ok && lastCast > 0 {
			now := float64(time.Now().UnixMilli()) / 1000.0
			elapsed := now - lastCast
			cd := spellDef.Cooldown
			if elapsed < cd {
				pct := 1.0 - (elapsed / cd)
				h := sw.SlotSize * pct
				ebitenutil.DrawRect(screen, sx, sy+sw.SlotSize-h, sw.SlotSize, h, color.RGBA{0, 0, 0, 150})
			}
		}
	}

	// Tooltip handling moved to UISystem
}

func (sw *SpellsWidget) IsHovered(mx, my int) bool {
	return float64(mx) >= sw.X && float64(mx) <= sw.X+sw.Width && float64(my) >= sw.Y && float64(my) <= sw.Y+sw.Height
}

func (sw *SpellsWidget) HandleInput(x, y int) bool {
	return sw.IsHovered(x, y)
}

type EquipmentWidget struct {
	BaseElement
	Slots       [9]string // Item IDs
	SlotSize    float64
	SlotOffsets [9]struct{ X, Y float64 }
	HiddenIndex int

	OnSlotRightClick func(slotIndex int, mx, my int)
}

func NewEquipmentWidget(x, y float64) *EquipmentWidget {
	ew := &EquipmentWidget{
		BaseElement: BaseElement{X: x, Y: y, Width: 200, Height: 200, Visible: true},
		SlotSize:    40,
		HiddenIndex: -1,
	}

	// Define positions relative to widget X, Y
	// Column 1 (x=40): Weapon(5), Hands(8)
	// Column 2 (x=80): Head(0), Neck(1), Body(3), Legs(4), Feet(7)
	// Column 3 (x=120): Back(2), Shield(6)

	ew.SlotOffsets[0] = struct{ X, Y float64 }{80, 0}   // Head
	ew.SlotOffsets[1] = struct{ X, Y float64 }{80, 40}  // Neck
	ew.SlotOffsets[2] = struct{ X, Y float64 }{120, 40} // Back
	ew.SlotOffsets[3] = struct{ X, Y float64 }{80, 80}  // Body
	ew.SlotOffsets[4] = struct{ X, Y float64 }{80, 120} // Legs
	ew.SlotOffsets[5] = struct{ X, Y float64 }{40, 80}  // Weapon
	ew.SlotOffsets[6] = struct{ X, Y float64 }{120, 80} // Shield
	ew.SlotOffsets[7] = struct{ X, Y float64 }{80, 160} // Feet
	ew.SlotOffsets[8] = struct{ X, Y float64 }{40, 120} // Hands

	return ew
}

func (ew *EquipmentWidget) Update() (bool, error) {
	if !ew.Visible {
		return false, nil
	}
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		idx := ew.GetSlotAt(mx, my)
		if idx != -1 && ew.Slots[idx] != "" {
			if ew.OnSlotRightClick != nil {
				ew.OnSlotRightClick(idx, mx, my)
			}
			return true, nil
		}
	}
	return false, nil
}

func (ew *EquipmentWidget) Draw(screen *ebiten.Image) {
	if !ew.Visible {
		return
	}

	for i, itemID := range ew.Slots {
		sx := ew.X + ew.SlotOffsets[i].X
		sy := ew.Y + ew.SlotOffsets[i].Y

		// Slot Background
		ebitenutil.DrawRect(screen, sx+1, sy+1, ew.SlotSize-2, ew.SlotSize-2, color.RGBA{60, 60, 60, 255})

		// Item
		if itemID != "" && i != ew.HiddenIndex {
			ebitenutil.DrawRect(screen, sx+5, sy+5, ew.SlotSize-10, ew.SlotSize-10, color.RGBA{100, 200, 100, 255})
			ebitenutil.DebugPrintAt(screen, itemID[:1], int(sx+10), int(sy+10))
		}

		// Border
		ebitenutil.DrawLine(screen, sx, sy, sx+ew.SlotSize, sy, color.Gray{100})
		ebitenutil.DrawLine(screen, sx, sy, sx, sy+ew.SlotSize, color.Gray{100})
	}
}

func (ew *EquipmentWidget) GetSlotAt(mx, my int) int {
	for i, off := range ew.SlotOffsets {
		sx := ew.X + off.X
		sy := ew.Y + off.Y
		if float64(mx) >= sx && float64(mx) <= sx+ew.SlotSize && float64(my) >= sy && float64(my) <= sy+ew.SlotSize {
			return i
		}
	}
	return -1
}

func (ew *EquipmentWidget) IsHovered(mx, my int) bool {
	return ew.GetSlotAt(mx, my) != -1
}

func (ew *EquipmentWidget) HandleInput(x, y int) bool {
	return ew.IsHovered(x, y)
}
