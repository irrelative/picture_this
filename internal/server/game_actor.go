package server

import "errors"

type actorCommand struct {
	apply  func(*Game) error
	result chan error
}

// gameActor is the serialization boundary for one active game. HTTP handlers,
// timers, and restore operations submit mutations instead of competing for a
// process-wide store lock.
type gameActor struct {
	game     *Game
	commands chan actorCommand
}

func newGameActor(game *Game) *gameActor {
	actor := &gameActor{game: game, commands: make(chan actorCommand, 64)}
	go actor.run()
	return actor
}

func (a *gameActor) run() {
	for command := range a.commands {
		if command.apply == nil {
			command.result <- errors.New("empty game command")
			continue
		}
		err := command.apply(a.game)
		if err == nil {
			a.game.Version++
		}
		command.result <- err
	}
}

func (a *gameActor) execute(apply func(*Game) error) error {
	result := make(chan error, 1)
	a.commands <- actorCommand{apply: apply, result: result}
	return <-result
}
