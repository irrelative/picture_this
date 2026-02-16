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
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	openAIUserPlaceholder            = "{{instructions}}"
	openAIPromptCountPlaceholder     = "{{count}}"
	openAIAbsurdMinPlaceholder       = "{{absurd_min}}"
	openAIShortMinPlaceholder        = "{{short_min}}"
	openAIAnimalChoreMaxPlaceholder  = "{{animal_chore_max}}"
	openAINonAnimalMinPlaceholder    = "{{non_animal_min}}"
	openAIConceptFirstMinPlaceholder = "{{concept_first_min}}"
)

type GeneratedPrompt struct {
	Text string
	Joke string
}

type openAIChatRequest struct {
	Model               string              `json:"model"`
	Messages            []openAIChatMessage `json:"messages"`
	Temperature         float64             `json:"temperature,omitempty"`
	MaxTokens           int                 `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                 `json:"max_completion_tokens,omitempty"`
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

func (s *Server) generatePromptsFromOpenAI(ctx context.Context, instructions string, count int) ([]GeneratedPrompt, error) {
	if strings.TrimSpace(s.cfg.OpenAIAPIKey) == "" {
		return nil, errors.New("OpenAI API key is not configured.")
	}
	if count < minPromptGenerateCount || count > maxPromptGenerateCount {
		count = defaultPromptGenerateCount
	}
	systemPrompt, err := readPromptFile(s.cfg.OpenAIPromptSystemPath)
	if err != nil {
		return nil, err
	}
	systemPrompt = applyPromptCountPlaceholders(systemPrompt, count)
	userTemplate, err := readPromptFile(s.cfg.OpenAIPromptUserPath)
	if err != nil {
		return nil, err
	}
	userPrompt := strings.ReplaceAll(userTemplate, openAIUserPlaceholder, instructions)
	userPrompt = strings.ReplaceAll(userPrompt, openAIPromptCountPlaceholder, strconv.Itoa(count))

	model := strings.TrimSpace(s.cfg.OpenAIModel)
	maxResponseTokens := promptGenerationMaxTokens(count)
	reqBody := openAIChatRequest{
		Model: model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.9,
	}
	if requiresMaxCompletionTokens(model) {
		reqBody.MaxCompletionTokens = maxResponseTokens
	} else {
		reqBody.MaxTokens = maxResponseTokens
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
		if msg := parseOpenAIErrorMessage(body); msg != "" {
			return nil, fmt.Errorf("OpenAI request failed (%d): %s", resp.StatusCode, msg)
		}
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

	prompts := parsePromptList(parsed.Choices[0].Message.Content, count)
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

func parsePromptList(raw string, maxCount int) []GeneratedPrompt {
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
	return sanitizePromptList(out, maxCount)
}

func sanitizePromptList(prompts []GeneratedPrompt, maxCount int) []GeneratedPrompt {
	if maxCount < minPromptGenerateCount || maxCount > maxPromptGenerateCount {
		maxCount = defaultPromptGenerateCount
	}
	unique := make(map[string]struct{}, len(prompts))
	out := make([]GeneratedPrompt, 0, len(prompts))
	for _, prompt := range prompts {
		clean := strings.TrimSpace(prompt.Text)
		if clean == "" {
			continue
		}
		if !isAllowedGeneratedPrompt(clean) {
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
		if len(out) == maxCount {
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

func parseOpenAIErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error != nil {
		return strings.TrimSpace(envelope.Error.Message)
	}
	return strings.TrimSpace(string(body))
}

func requiresMaxCompletionTokens(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-5")
}

func promptGenerationMaxTokens(count int) int {
	estimated := 500 + (count * 30)
	if estimated < 700 {
		return 700
	}
	if estimated > 4000 {
		return 4000
	}
	return estimated
}

func applyPromptCountPlaceholders(systemPrompt string, count int) string {
	absurdMin := ceilPercent(count, 30)
	shortMin := ceilPercent(count, 80)
	animalChoreMax := count / 20
	if animalChoreMax < 1 {
		animalChoreMax = 1
	}
	nonAnimalMin := ceilPercent(count, 60)
	conceptFirstMin := count / 2
	if conceptFirstMin < 1 {
		conceptFirstMin = 1
	}

	replacements := map[string]string{
		openAIPromptCountPlaceholder:     strconv.Itoa(count),
		openAIAbsurdMinPlaceholder:       strconv.Itoa(absurdMin),
		openAIShortMinPlaceholder:        strconv.Itoa(shortMin),
		openAIAnimalChoreMaxPlaceholder:  strconv.Itoa(animalChoreMax),
		openAINonAnimalMinPlaceholder:    strconv.Itoa(nonAnimalMin),
		openAIConceptFirstMinPlaceholder: strconv.Itoa(conceptFirstMin),
	}
	result := systemPrompt
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func ceilPercent(total, percent int) int {
	if total <= 0 || percent <= 0 {
		return 0
	}
	return (total*percent + 99) / 100
}

var (
	disallowedGeneratedPromptStart = regexp.MustCompile(`(?i)^(a|an|the|draw|sketch|picture)\b`)
	disallowedAnimalChorePrompt    = regexp.MustCompile(`(?i)^(?:[a-z]+ly\s+)?(?:cat|dog|bear|penguin|squirrel|rabbit|monkey|lion|tiger|panda|otter|duck|fox|wolf|cow|horse|goat|chicken|frog|shark|whale|octopus)\s+(?:baking|cooking|cleaning|washing|wearing|riding|holding|eating|drinking|reading|writing|shopping|juggling)\b`)
)

func isAllowedGeneratedPrompt(prompt string) bool {
	clean := strings.TrimSpace(prompt)
	if clean == "" {
		return false
	}
	if disallowedGeneratedPromptStart.MatchString(clean) {
		return false
	}
	return !disallowedAnimalChorePrompt.MatchString(clean)
}
