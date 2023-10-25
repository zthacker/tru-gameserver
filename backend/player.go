package backend

type Player struct {
	IdentifierBase
	Name      string
	Level     int
	Character PlayerCharacter
}

type PlayerCharacter struct {
	HP     int
	MP     int
	Health int
}
