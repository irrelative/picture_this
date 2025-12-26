#!/usr/bin/env python3
"""End-to-end game smoke test without a browser."""

import argparse
import json
import os
import time
import urllib.error
import urllib.request

DEFAULT_BASE_URL = "http://localhost:8080"
PNG_1X1 = (
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8"
    "/x8AAwMBAp4pWZkAAAAASUVORK5CYII="
)
AVATAR_IMAGES = [
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4n5wMAAQqAcbUp9SsAAAAAElFTkSuQmCC",
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGMIPJ8GAAL7AYdeG79/AAAAAElFTkSuQmCC",
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGPwmvAIAALkAb2JlVB1AAAAAElFTkSuQmCC",
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4elQCAASFAdNllrGDAAAAAElFTkSuQmCC",
]
AVATAR_UPDATES = [
    AVATAR_IMAGES[2],
    AVATAR_IMAGES[3],
    AVATAR_IMAGES[0],
    AVATAR_IMAGES[1],
]


def request_json(method, url, payload=None):
    data = None
    headers = {}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            body = resp.read()
            if body:
                return resp.status, json.loads(body.decode("utf-8"))
            return resp.status, {}
    except urllib.error.HTTPError as exc:
        body = exc.read()
        if body:
            return exc.code, json.loads(body.decode("utf-8"))
        return exc.code, {}


def request_json_with_retry(method, url, payload=None, retries=10, delay=0.2):
    for attempt in range(retries):
        try:
            return request_json(method, url, payload)
        except urllib.error.URLError:
            if attempt >= retries - 1:
                raise
            time.sleep(delay)


def normalize_phase(phase):
    if phase == "votes":
        return "guesses-votes"
    return phase


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", DEFAULT_BASE_URL))
    parser.add_argument("--sleep", type=float, default=0.0)
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")

    status, game = request_json_with_retry("POST", f"{base_url}/api/games")
    assert status == 201, f"create game failed: {status} {game}"
    game_id = game["game_id"]
    print(f"Created game {game_id} join_code={game['join_code']}")

    status, categories = request_json("GET", f"{base_url}/api/prompts/categories")
    assert status == 200, f"categories failed: {status} {categories}"

    player_names = ["Alice", "Bob", "Carol", "Dave"]
    players = []
    for idx, name in enumerate(player_names):
        avatar_data = f"data:image/png;base64,{AVATAR_IMAGES[idx % len(AVATAR_IMAGES)]}"
        updated_avatar_data = f"data:image/png;base64,{AVATAR_UPDATES[idx % len(AVATAR_UPDATES)]}"
        status, player = request_json(
            "POST",
            f"{base_url}/api/games/{game_id}/join",
            {"name": name, "avatar_data": avatar_data},
        )
        assert status == 200, f"join {name} failed: {status} {player}"
        players.append(player)
        status, _ = request_json(
            "POST",
            f"{base_url}/api/games/{game_id}/avatar",
            {"player_id": player["player_id"], "avatar_data": updated_avatar_data},
        )
        assert status == 200, f"avatar update failed for {name}"
    joined = " ".join(f"{player_names[idx]}={player['player_id']}" for idx, player in enumerate(players))
    print(f"Joined players: {joined}")

    status, _ = request_json(
        "POST",
        f"{base_url}/api/games/{game_id}/settings",
        {"player_id": players[0]["player_id"], "rounds": 2, "max_players": 0, "prompt_category": "", "lobby_locked": False},
    )
    assert status == 200, f"settings update failed: {status}"

    status, started = request_json(
        "POST",
        f"{base_url}/api/games/{game_id}/start",
        {"player_id": players[0]["player_id"]},
    )
    assert status == 200, f"start game failed: {status} {started}"
    print(f"Started game phase={started.get('phase')}")

    status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
    assert status == 200, "snapshot failed"
    total_rounds = int(snapshot.get("total_rounds", 1))

    def fetch_prompt(player_id):
        return request_json(
            "GET", f"{base_url}/api/games/{game_id}/players/{player_id}/prompt"
        )

    def submit_drawing(player_id, prompt_text):
        drawing_payload = {
            "image_data": f"data:image/png;base64,{PNG_1X1}",
            "player_id": player_id,
            "prompt": prompt_text,
        }
        return request_json(
            "POST",
            f"{base_url}/api/games/{game_id}/drawings",
            drawing_payload,
        )

    def guess_for(player_id, text):
        return request_json(
            "POST",
            f"{base_url}/api/games/{game_id}/guesses",
            {"player_id": player_id, "guess": text},
        )

    for round_number in range(1, total_rounds + 1):
        prompts = {}
        for player in players:
            status, prompt = fetch_prompt(player["player_id"])
            assert status == 200, "failed to fetch prompts"
            prompts[player["player_id"]] = prompt["prompt"]

        for player in players:
            status, _ = submit_drawing(player["player_id"], prompts[player["player_id"]])
            assert status == 200, f"drawing failed for player {player['player_id']}"
        print(f"Submitted drawings for round {round_number}")

        time.sleep(args.sleep)
        status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
        assert status == 200, "snapshot failed"
        phase = normalize_phase(snapshot.get("phase"))
        assert phase == "guesses", f"expected guesses phase, got {snapshot.get('phase')}"

        for drawing_index in range(len(players)):
            guard = 0
            max_guess_turns = max(10, len(players) * len(players) * 2)
            while normalize_phase(snapshot.get("phase")) == "guesses" and guard < max_guess_turns:
                guard += 1
                turn = snapshot.get("guess_turn")
                assert turn, "no guess turn found"
                guesser = turn["guesser_id"]
                status, _ = guess_for(guesser, f"guess-{round_number}-{drawing_index}-{guard}")
                assert status == 200, f"guess failed for player {guesser}"
                status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
                assert status == 200, "snapshot failed"

            phase = normalize_phase(snapshot.get("phase"))
            assert phase == "guesses-votes", f"expected guesses-votes phase, got {snapshot.get('phase')}"

            guard = 0
            while normalize_phase(snapshot.get("phase")) == "guesses-votes" and guard < 20:
                guard += 1
                turn = snapshot.get("vote_turn")
                assert turn, "no vote turn found"
                voter = turn["voter_id"]
                options = turn.get("options") or []
                assert options, "no vote options"
                choice = options[0]
                status, _ = request_json(
                    "POST",
                    f"{base_url}/api/games/{game_id}/votes",
                    {"player_id": voter, "choice": choice},
                )
                if status != 200:
                    _, detail = request_json("GET", f"{base_url}/api/games/{game_id}")
                    raise AssertionError(
                        f"vote failed for player {voter}: status={status} snapshot_phase={detail.get('phase')}"
                    )
                status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
                assert status == 200, "snapshot failed"

            phase = normalize_phase(snapshot.get("phase"))
            if phase == "results":
                status, snapshot = request_json("POST", f"{base_url}/api/games/{game_id}/advance", {})
                assert status == 200, f"advance failed: {snapshot}"
                phase = normalize_phase(snapshot.get("phase"))
                if phase == "results":
                    status, snapshot = request_json(
                        "POST", f"{base_url}/api/games/{game_id}/advance", {}
                    )
                    assert status == 200, f"advance failed: {snapshot}"
                    phase = normalize_phase(snapshot.get("phase"))

            if drawing_index < len(players) - 1:
                assert phase == "guesses", f"expected guesses phase, got {snapshot.get('phase')}"
            else:
                if round_number < total_rounds:
                    assert phase == "drawings", f"expected drawings phase, got {snapshot.get('phase')}"
                else:
                    assert phase == "complete", f"expected complete phase, got {snapshot.get('phase')}"
                    print("Votes submitted")

    status, results = request_json("GET", f"{base_url}/api/games/{game_id}/results")
    assert status == 200, f"results failed: {results}"
    print("Results:")
    print(json.dumps(results, indent=2))

    status, events = request_json("GET", f"{base_url}/api/games/{game_id}/events")
    assert status == 200, f"events failed: {events}"

    print("E2E run complete")


if __name__ == "__main__":
    main()
