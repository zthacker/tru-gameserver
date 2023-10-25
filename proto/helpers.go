package proto

import (
	"gameserver/backend"
	"log"
)

func GetProtoEntity(entity backend.Identifier) *Entity {
	switch entity.(type) {
	case *backend.Player:
		player := entity.(*backend.Player)
		protoPlayer := Entity_Player{
			Player: GetProtoPlayer(player),
		}
		return &Entity{Entity: &protoPlayer}
	}
	log.Printf("cannot get proto entity for %T -> %+v", entity, entity)
	return nil
}

func GetProtoPlayer(player *backend.Player) *Player {
	return &Player{
		Id:   player.ID().String(),
		Name: player.Name,
	}
}
