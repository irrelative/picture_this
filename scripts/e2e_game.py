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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", DEFAULT_BASE_URL))
    parser.add_argument("--sleep", type=float, default=0.0)
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")

    status, game = request_json("POST", f"{base_url}/api/games")
    assert status == 201, f"create game failed: {status} {game}"
    game_id = game["game_id"]
    print(f"Created game {game_id} join_code={game['join_code']}")

    status, alice = request_json("POST", f"{base_url}/api/games/{game_id}/join", {"name": "Alice"})
    assert status == 200, f"join Alice failed: {status} {alice}"
    status, bob = request_json("POST", f"{base_url}/api/games/{game_id}/join", {"name": "Bob"})
    assert status == 200, f"join Bob failed: {status} {bob}"
    print(f"Joined players: Alice={alice['player_id']} Bob={bob['player_id']}")

    status, started = request_json("POST", f"{base_url}/api/games/{game_id}/start", {})
    assert status == 200, f"start game failed: {status} {started}"
    print(f"Started game phase={started.get('phase')}")

    status, alice_prompt = request_json("GET", f"{base_url}/api/games/{game_id}/players/{alice['player_id']}/prompt")
    status2, bob_prompt = request_json("GET", f"{base_url}/api/games/{game_id}/players/{bob['player_id']}/prompt")
    assert status == 200 and status2 == 200, "failed to fetch prompts"

    alice_prompt_text = alice_prompt["prompts"][0]
    bob_prompt_text = bob_prompt["prompts"][0]

    drawing_payload = {
        "image_data": f"data:image/png;base64,{PNG_1X1}",
    }
    status, _ = request_json(
        "POST",
        f"{base_url}/api/games/{game_id}/drawings",
        {**drawing_payload, "player_id": alice["player_id"], "prompt": alice_prompt_text},
    )
    assert status == 200, "Alice drawing failed"
    status, _ = request_json(
        "POST",
        f"{base_url}/api/games/{game_id}/drawings",
        {**drawing_payload, "player_id": bob["player_id"], "prompt": bob_prompt_text},
    )
    assert status == 200, "Bob drawing failed"
    print("Submitted drawings")

    time.sleep(args.sleep)
    status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
    assert status == 200, "snapshot failed"
    assert snapshot.get("phase") == "guesses", f"expected guesses phase, got {snapshot.get('phase')}"

    def guess_for(player_id, text):
        return request_json(
            "POST",
            f"{base_url}/api/games/{game_id}/guesses",
            {"player_id": player_id, "guess": text},
        )

    turn = snapshot.get("guess_turn")
    assert turn, "no guess turn found"
    guesser = turn["guesser_id"]
    status, _ = guess_for(guesser, "wild guess")
    assert status == 200, f"guess failed for player {guesser}"

    status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
    turn = snapshot.get("guess_turn")
    assert turn, "missing next guess turn"
    guesser = turn["guesser_id"]
    status, _ = guess_for(guesser, "another guess")
    assert status == 200, f"guess failed for player {guesser}"

    status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
    assert snapshot.get("phase") == "votes", f"expected votes phase, got {snapshot.get('phase')}"

    status, vote = request_json(
        "POST",
        f"{base_url}/api/games/{game_id}/votes",
        {"player_id": alice["player_id"], "guess": "wild guess"},
    )
    assert status == 200, f"vote failed: {vote}"
    print("Votes submitted")

    status, results = request_json("GET", f"{base_url}/api/games/{game_id}/results")
    assert status == 200, f"results failed: {results}"
    print("Results:")
    print(json.dumps(results, indent=2))

    print("E2E run complete")


if __name__ == "__main__":
    main()
