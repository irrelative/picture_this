package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func Home() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, _ = io.WriteString(w, `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>Picture This</title>
    <link rel="stylesheet" href="/static/styles.css"/>
  </head>
  <body>
    <main class="shell">
      <header class="hero">
        <span class="tag">Picture This</span>
        <h1>Draw together. Guess boldly.</h1>
        <p>Host a game in seconds or jump into a session with your code.</p>
      </header>

      <section class="panel">
        <div>
          <h2>Create a game</h2>
          <p>Generate a new lobby and share the join code with your players.</p>
        </div>
        <button id="createGame" class="primary">Create game</button>
        <div id="createResult" class="result"></div>
      </section>

      <section class="panel">
        <div>
          <h2>Join a game</h2>
          <p>Enter the join code from the host and your display name.</p>
        </div>
        <form id="joinForm" class="join-form">
          <input name="code" placeholder="Join code" autocomplete="off" required/>
          <input name="name" placeholder="Display name" autocomplete="name" required/>
          <button type="submit" class="secondary">Join game</button>
        </form>
        <div id="joinResult" class="result"></div>
      </section>
    </main>

    <script>
      const createBtn = document.getElementById("createGame");
      const createResult = document.getElementById("createResult");
      const joinForm = document.getElementById("joinForm");
      const joinResult = document.getElementById("joinResult");

      createBtn.addEventListener("click", async () => {
        createResult.textContent = "Creating game...";
        const res = await fetch("/api/games", { method: "POST" });
        const data = await res.json();
        if (!res.ok) {
          createResult.textContent = data.error || "Failed to create game.";
          return;
        }
        createResult.textContent = "Game created. Join code: " + data.join_code;
      });

      joinForm.addEventListener("submit", async (event) => {
        event.preventDefault();
        joinResult.textContent = "Joining game...";
        const code = joinForm.elements.code.value.trim();
        const name = joinForm.elements.name.value.trim();
        const res = await fetch("/api/games/" + encodeURIComponent(code) + "/join", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name })
        });
        const data = await res.json();
        if (!res.ok) {
          joinResult.textContent = data.error || "Failed to join game.";
          return;
        }
        joinResult.textContent = "Joined game " + data.game_id + " as " + data.player + ".";
      });
    </script>
  </body>
</html>
`)
		return nil
	})
}
