package config

import "testing"

func TestDefaultUsesFrontierPromptGenerationModel(t *testing.T) {
	if got := Default().OpenAIModel; got != "gpt-5.6" {
		t.Fatalf("expected frontier prompt generation model gpt-5.6, got %q", got)
	}
}
