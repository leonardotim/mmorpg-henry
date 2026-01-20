package items

func init() {
	// Potions, Food, etc will go here
	Register(ItemDefinition{
		ID:            "potion_health_small",
		Name:          "Small Health Potion",
		Type:          ItemTypeConsumable,
		Description:   "Restores a small amount of health.",
		EquipmentSlot: -1,
	})
}
