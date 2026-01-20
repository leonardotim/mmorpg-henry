package ecs

import (
	"reflect"
	"sync/atomic"
)

// Entity is a unique identifier for a game object.
type Entity uint64

// System is logic that operates on entities with specific components.
type System interface {
	Update(dt float64)
}

// Component is a marker interface for data attached to entities.
type Component interface{}

// World manages entities and their components.
type World struct {
	nextEntityID uint64
	// components maps ComponentType -> EntityID -> Component
	components map[reflect.Type]map[Entity]Component
	systems    []System
}

func NewWorld() *World {
	return &World{
		components: make(map[reflect.Type]map[Entity]Component),
		systems:    make([]System, 0),
	}
}

// NewEntity creates a new entity with a unique ID.
func (w *World) NewEntity() Entity {
	id := atomic.AddUint64(&w.nextEntityID, 1)
	return Entity(id)
}

// RemoveEntity removes all components associated with an entity.
func (w *World) RemoveEntity(e Entity) {
	for _, store := range w.components {
		delete(store, e)
	}
}

// AddComponent attaches a component to an entity.
func (w *World) AddComponent(e Entity, c Component) {
	cType := reflect.TypeOf(c)
	if _, ok := w.components[cType]; !ok {
		w.components[cType] = make(map[Entity]Component)
	}
	w.components[cType][e] = c
}

// RemoveComponent removes a component of type T from an entity.
// It uses reflection on the passed 'c' argument to determine the type.
func (w *World) RemoveComponent(e Entity, c Component) {
	cType := reflect.TypeOf(c)
	if store, ok := w.components[cType]; ok {
		delete(store, e)
	}
}

// GetComponent retrieves a component of type T for an entity.
// It uses a helper function concept since Go generics don't allow method type parameters easily on struct methods yet in this way without wrapper.
// But we can make a Generic helper function outside or use reflection wrapper.
// Let's use a helper method that takes a pointer to the component type for reflection key,
// OR we can make this generic if we use a free function or if we are on Go 1.18+.
// Go 1.18+ supports generics.
func GetComponent[T Component](w *World, e Entity) (*T, bool) {
	var zero T
	cType := reflect.TypeOf(zero)
	if store, ok := w.components[cType]; ok {
		if val, ok := store[e]; ok {
			castVal := val.(T)
			return &castVal, true
		}
	}
	return nil, false
}

// AddSystem adds a system to the world.
func (w *World) AddSystem(s System) {
	w.systems = append(w.systems, s)
}

// Update updates all systems.
func (w *World) Update(dt float64) {
	for _, system := range w.systems {
		system.Update(dt)
	}
}

// Query returns all entities that have a specific component type.
func Query[T Component](w *World) []Entity {
	var zero T
	cType := reflect.TypeOf(zero)
	var entities []Entity
	if store, ok := w.components[cType]; ok {
		for e := range store {
			entities = append(entities, e)
		}
	}
	return entities
}
