package backend

import (
	"github.com/google/uuid"
	"sync"
)

type Game struct {
	Entities map[uuid.UUID]Identifier
	Mu       sync.RWMutex
}

func (g *Game) AddEntity(entity Identifier) {
	g.Entities[entity.ID()] = entity
}

func (g *Game) UpdateEntity(entity Identifier) {
	g.Entities[entity.ID()] = entity

}

func (g *Game) RemoveEntity(id uuid.UUID) {
	delete(g.Entities, id)
}

func (g *Game) GetEntity(id uuid.UUID) Identifier {
	return g.Entities[id]
}

func NewGame() *Game {
	return &Game{Entities: make(map[uuid.UUID]Identifier)}
}

type Identifier interface {
	ID() uuid.UUID
}

type IdentifierBase struct {
	UUID uuid.UUID
}

func (e IdentifierBase) ID() uuid.UUID {
	return e.UUID
}
