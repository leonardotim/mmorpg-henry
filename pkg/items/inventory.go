package items

import (
	"errors"
	"henry/pkg/shared/components"
)

// NewInventory creates a new inventory component with specified capacity
// Returns an initialized component (not pointer yet, usually ECS takes value or pointer)
// Our ECS uses empty interface, usually we store pointers for mutability.
func NewInventory(capacity int) *components.InventoryComponent {
	return &components.InventoryComponent{
		Slots:    make([]components.InventorySlot, capacity),
		Capacity: capacity,
	}
}

// AddItem adds an item to the inventory.
// Tries to stack first, then find empty slot.
func AddItem(inv *components.InventoryComponent, itemID string, quantity int) error {
	// 1. Try to stack
	// Logic: Iterate slots, if same ID, add.
	// NOTE: We assume infinite stack size for now or need MaxStack in ItemDefinition

	// Check if item exists
	if _, ok := Registry[itemID]; !ok {
		return errors.New("item not defined: " + itemID)
	}

	for i := range inv.Slots {
		if inv.Slots[i].ItemID == itemID {
			inv.Slots[i].Quantity += quantity
			return nil
		}
	}

	// 2. Find empty slot
	for i := range inv.Slots {
		if inv.Slots[i].ItemID == "" || inv.Slots[i].Quantity == 0 {
			inv.Slots[i].ItemID = itemID
			inv.Slots[i].Quantity = quantity
			return nil
		}
	}

	return errors.New("inventory full")
}

// RemoveItem removes a quantity of item from a specific slot
func RemoveItem(inv *components.InventoryComponent, slotIndex int, quantity int) error {
	if slotIndex < 0 || slotIndex >= len(inv.Slots) {
		return errors.New("invalid slot index")
	}

	slot := &inv.Slots[slotIndex]
	if slot.Quantity < quantity {
		return errors.New("not enough items")
	}

	slot.Quantity -= quantity
	if slot.Quantity <= 0 {
		slot.ItemID = ""
		slot.Quantity = 0
	}
	return nil
}

// SwapItems swaps content of two slots
func SwapItems(inv *components.InventoryComponent, slotA, slotB int) error {
	if slotA < 0 || slotA >= len(inv.Slots) || slotB < 0 || slotB >= len(inv.Slots) {
		return errors.New("invalid slot index")
	}

	inv.Slots[slotA], inv.Slots[slotB] = inv.Slots[slotB], inv.Slots[slotA]
	return nil
}

// GetSlot returns the generic slot data
func GetSlot(inv *components.InventoryComponent, slotIndex int) (components.InventorySlot, error) {
	if slotIndex < 0 || slotIndex >= len(inv.Slots) {
		return components.InventorySlot{}, errors.New("invalid slot index")
	}
	return inv.Slots[slotIndex], nil
}
