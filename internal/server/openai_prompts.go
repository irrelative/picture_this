package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const openAIUserPlaceholder = "{{instructions}}"

type GeneratedPrompt struct {
	Text string
	Joke string
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *Server) generatePromptsFromOpenAI(ctx context.Context, instructions string) ([]GeneratedPrompt, error) {
	if strings.TrimSpace(s.cfg.OpenAIAPIKey) == "" {
		return nil, errors.New("OpenAI API key is not configured.")
	}
	systemPrompt, err := readPromptFile(s.cfg.OpenAIPromptSystemPath)
	if err != nil {
		return nil, err
	}
	userTemplate, err := readPromptFile(s.cfg.OpenAIPromptUserPath)
	if err != nil {
		return nil, err
	}
	userPrompt := strings.ReplaceAll(userTemplate, openAIUserPlaceholder, instructions)

	reqBody := openAIChatRequest{
		Model: s.cfg.OpenAIModel,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.9,
		MaxTokens:   700,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAI request")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAI request")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.cfg.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach OpenAI")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAI response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAI request failed (%d)", resp.StatusCode)
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response")
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("OpenAI error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("OpenAI returned no prompt choices.")
	}

	prompts := parsePromptList(parsed.Choices[0].Message.Content)
	if len(prompts) == 0 {
		return nil, errors.New("OpenAI did not return prompts in the expected format.")
	}
	return prompts, nil
}

func readPromptFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt template: %s", path)
	}
	return strings.TrimSpace(string(content)), nil
}

func parsePromptList(raw string) []GeneratedPrompt {
	lines := strings.Split(raw, "\n")
	out := make([]GeneratedPrompt, 0, len(lines))
	var current *GeneratedPrompt
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*•")
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789.")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "joke:") {
			if current != nil {
				current.Joke = strings.TrimSpace(strings.TrimPrefix(line, "Joke:"))
				current.Joke = strings.TrimSpace(strings.TrimPrefix(current.Joke, "joke:"))
			}
			continue
		}
		line = stripDifficultyTag(line)
		entry := GeneratedPrompt{Text: line}
		out = append(out, entry)
		current = &out[len(out)-1]
	}
	return sanitizePromptList(out)
}

func sanitizePromptList(prompts []GeneratedPrompt) []GeneratedPrompt {
	unique := make(map[string]struct{}, len(prompts))
	out := make([]GeneratedPrompt, 0, len(prompts))
	for _, prompt := range prompts {
		clean := strings.TrimSpace(prompt.Text)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = struct{}{}
		prompt.Text = clean
		prompt.Joke = strings.TrimSpace(prompt.Joke)
		out = append(out, prompt)
		if len(out) == 20 {
			break
		}
	}
	return out
}

func stripDifficultyTag(prompt string) string {
	clean := strings.TrimSpace(prompt)
	tags := []string{"[E]", "[M]", "[H]", "[A]"}
	for _, tag := range tags {
		if strings.HasPrefix(clean, tag) {
			return strings.TrimSpace(strings.TrimPrefix(clean, tag))
		}
	}
	return clean
}
