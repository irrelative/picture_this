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
		{Text: "A bear baking cookies"},
		{Text: "bear baking cookies"},
		{Text: "Draw a suspiciously buff librarian"},
		{Text: "Suspiciously buff librarian"},
		{Text: "Midlife crisis at a wizard school"},
	}

	got := sanitizePromptList(input)
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
