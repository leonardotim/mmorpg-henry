package items

func init() {
	// Crafting materials, quest items, etc.
	Register(ItemDefinition{
		ID:          "coin_gold",
		Name:        "Gold Coin",
		Type:        ItemTypeMisc,
		Description: "Standard currency.",
	})
}
