package server

import "errors"

type actorCommand struct {
	apply   func(*Game) error
	persist func(*Game) error
	result  chan error
	read    chan *Game
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
		if command.read != nil {
			command.read <- cloneGame(a.game)
			continue
		}
		if command.apply == nil {
			command.result <- errors.New("empty game command")
			continue
		}
		candidate := cloneGame(a.game)
		err := command.apply(candidate)
		if err == nil {
			candidate.Version++
			if command.persist != nil {
				err = command.persist(candidate)
			}
		}
		if err == nil {
			a.game = candidate
		}
		command.result <- err
	}
}

func (a *gameActor) execute(apply func(*Game) error) error {
	result := make(chan error, 1)
	a.commands <- actorCommand{apply: apply, result: result}
	return <-result
}

func (a *gameActor) executeDurably(apply, persist func(*Game) error) error {
	result := make(chan error, 1)
	a.commands <- actorCommand{apply: apply, persist: persist, result: result}
	return <-result
}

func (a *gameActor) snapshot() *Game {
	result := make(chan *Game, 1)
	a.commands <- actorCommand{read: result}
	return <-result
}
