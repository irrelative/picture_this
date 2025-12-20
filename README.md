# Picture This

This is a port of the game Drawful, as best as possible, using the following technology:

* Golang backend (Go 1.25.5)
* Golang templ templating for WebUI and mobile interfaces
* Websockets used for server and client syncing
* Minimal javascipt, no frameworks
* WebUI frontend for managing the game, showing "secret" code
* Postgres for backend state storage of game, drawings, etc. The game should be able to crash and be restarted without losing game state.


For now, don't include:
* Sound effects/music
* Voiceover. Instead, have the instructions printed to the WebUI
