package systems

import (
	"henry/pkg/shared/components"
	"henry/pkg/shared/ecs"
	protocol "henry/pkg/shared/network"
)

type NetworkSystem struct {
	World *ecs.World
}

func NewNetworkSystem(world *ecs.World) *NetworkSystem {
	return &NetworkSystem{
		World: world,
	}
}

func (s *NetworkSystem) PrepareStateUpdate() protocol.Packet {
	snapshot := protocol.StateUpdatePacket{
		Entities: make([]protocol.EntitySnapshot, 0),
	}

	entities := ecs.Query[components.TransformComponent](s.World)
	for _, id := range entities {
		trans, _ := ecs.GetComponent[components.TransformComponent](s.World, id)
		sprite, _ := ecs.GetComponent[components.SpriteComponent](s.World, id)
		stats, _ := ecs.GetComponent[components.StatsComponent](s.World, id)
		physics, _ := ecs.GetComponent[components.PhysicsComponent](s.World, id)

		if sprite != nil {
			snapshot.Entities = append(snapshot.Entities, protocol.EntitySnapshot{
				ID:        id,
				Transform: trans,
				Physics:   physics,
				Sprite:    sprite,
				Stats:     stats,
			})
		}
	}

	return protocol.Packet{
		Type: protocol.PacketStateUpdate,
		Data: snapshot,
	}
}
