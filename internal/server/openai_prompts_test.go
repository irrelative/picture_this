package server

import "testing"

func TestIsAllowedGeneratedPrompt(t *testing.T) {
	tests := []struct {
		name  string
		input string
		allow bool
	}{
		{name: "reject article prefix", input: "A bear baking cookies", allow: false},
		{name: "reject draw prefix", input: "Draw a haunted vending machine", allow: false},
		{name: "reject animal chore", input: "bear baking cookies", allow: false},
		{name: "reject animal wearing", input: "penguin wearing a backpack", allow: false},
		{name: "allow conceptual", input: "Midlife crisis at a wizard school", allow: true},
		{name: "allow role mismatch", input: "Overconfident ghost realtor", allow: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAllowedGeneratedPrompt(tc.input)
			if got != tc.allow {
				t.Fatalf("expected allow=%v got=%v for %q", tc.allow, got, tc.input)
			}
		})
	}
}

func TestSanitizePromptListFiltersDisallowedPatterns(t *testing.T) {
	input := []GeneratedPrompt{
		{Text: "A bear baking cookies", Joke: "No one asked for this catering plan."},
		{Text: "bear baking cookies", Joke: "The kitchen has lower standards now."},
		{Text: "Draw a suspiciously buff librarian", Joke: "Late fees build unusual strength."},
		{Text: "Suspiciously buff librarian", Joke: "Late fees build unusual strength."},
		{Text: "Midlife crisis at a wizard school", Joke: "The sports broom was financially irresponsible."},
	}

	got := sanitizePromptList(input, defaultPromptGenerateCount)
	if len(got) != 2 {
		t.Fatalf("expected 2 prompts after filtering, got %d", len(got))
	}
	if got[0].Text != "Suspiciously buff librarian" {
		t.Fatalf("unexpected prompt[0]: %q", got[0].Text)
	}
	if got[1].Text != "Midlife crisis at a wizard school" {
		t.Fatalf("unexpected prompt[1]: %q", got[1].Text)
	}
}

func TestParsePromptGenerateCount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "default", input: "", want: defaultPromptGenerateCount},
		{name: "min", input: "1", want: 1},
		{name: "max", input: "100", want: 100},
		{name: "too low", input: "0", want: defaultPromptGenerateCount, wantErr: true},
		{name: "too high", input: "101", want: defaultPromptGenerateCount, wantErr: true},
		{name: "invalid", input: "x", want: defaultPromptGenerateCount, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePromptGenerateCount(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected err=%v got err=%v", tc.wantErr, err != nil)
			}
			if got != tc.want {
				t.Fatalf("expected count %d, got %d", tc.want, got)
			}
		})
	}
}

func TestParsePromptListRequiresJokeForEveryPrompt(t *testing.T) {
	raw := `[E] Dentist hiding from a tooth
Joke: Professional confidence has officially left the building.
[M] Parking valet for shopping carts
[A] Knight regretting the tiny horse
Joke: The kingdom cut transportation costs again.`

	prompts := parsePromptList(raw, 10)
	if len(prompts) != 2 {
		t.Fatalf("expected 2 complete prompt pairs, got %d", len(prompts))
	}
	if prompts[0].Text != "Dentist hiding from a tooth" {
		t.Fatalf("unexpected first prompt: %q", prompts[0].Text)
	}
	if prompts[1].Joke != "The kingdom cut transportation costs again." {
		t.Fatalf("unexpected second joke: %q", prompts[1].Joke)
	}
}

func TestSanitizePromptListTrimsCompletePairs(t *testing.T) {
	prompts := sanitizePromptList([]GeneratedPrompt{
		{Text: "  Moon caught stealing the tide  ", Joke: "  Celestial crime remains mostly above the law.  "},
		{Text: "Emotional support traffic cone"},
	}, 10)

	if len(prompts) != 1 {
		t.Fatalf("expected 1 complete prompt pair, got %d", len(prompts))
	}
	if prompts[0].Text != "Moon caught stealing the tide" {
		t.Fatalf("unexpected prompt text: %q", prompts[0].Text)
	}
	if prompts[0].Joke != "Celestial crime remains mostly above the law." {
		t.Fatalf("unexpected joke: %q", prompts[0].Joke)
	}
}
