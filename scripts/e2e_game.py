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

    status, game = request_json("POST", f"{base_url}/api/games")
    assert status == 201, f"create game failed: {status} {game}"
    game_id = game["game_id"]
    print(f"Created game {game_id} join_code={game['join_code']}")

    status, alice = request_json("POST", f"{base_url}/api/games/{game_id}/join", {"name": "Alice"})
    assert status == 200, f"join Alice failed: {status} {alice}"
    status, bob = request_json("POST", f"{base_url}/api/games/{game_id}/join", {"name": "Bob"})
    assert status == 200, f"join Bob failed: {status} {bob}"
    print(f"Joined players: Alice={alice['player_id']} Bob={bob['player_id']}")

    status, started = request_json("POST", f"{base_url}/api/games/{game_id}/start", {"player_id": alice["player_id"]})
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
        status, alice_prompt = fetch_prompt(alice["player_id"])
        status2, bob_prompt = fetch_prompt(bob["player_id"])
        assert status == 200 and status2 == 200, "failed to fetch prompts"

        status, _ = submit_drawing(alice["player_id"], alice_prompt["prompt"])
        assert status == 200, "Alice drawing failed"
        status, _ = submit_drawing(bob["player_id"], bob_prompt["prompt"])
        assert status == 200, "Bob drawing failed"
        print(f"Submitted drawings for round {round_number}")

        time.sleep(args.sleep)
        status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
        assert status == 200, "snapshot failed"
        phase = normalize_phase(snapshot.get("phase"))
        assert phase == "guesses", f"expected guesses phase, got {snapshot.get('phase')}"

        guard = 0
        while normalize_phase(snapshot.get("phase")) == "guesses" and guard < 10:
            guard += 1
            turn = snapshot.get("guess_turn")
            assert turn, "no guess turn found"
            guesser = turn["guesser_id"]
            status, _ = guess_for(guesser, f"guess-{round_number}-{guard}")
            assert status == 200, f"guess failed for player {guesser}"
            status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
            assert status == 200, "snapshot failed"

        phase = normalize_phase(snapshot.get("phase"))
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
                raise AssertionError(f"vote failed for player {voter}: status={status} snapshot_phase={detail.get('phase')}")
            status, snapshot = request_json("GET", f"{base_url}/api/games/{game_id}")
            assert status == 200, "snapshot failed"

        phase = normalize_phase(snapshot.get("phase"))
        if round_number < total_rounds:
            assert phase == "drawings", f"expected drawings phase, got {snapshot.get('phase')}"
        else:
            assert phase == "results", f"expected results phase, got {snapshot.get('phase')}"
            print("Votes submitted")

    status, results = request_json("GET", f"{base_url}/api/games/{game_id}/results")
    assert status == 200, f"results failed: {results}"
    print("Results:")
    print(json.dumps(results, indent=2))

    print("E2E run complete")


if __name__ == "__main__":
    main()
