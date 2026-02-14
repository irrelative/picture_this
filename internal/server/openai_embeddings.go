package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const promptEmbeddingDimensions = 1536

type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *Server) generateEmbeddingsFromOpenAI(ctx context.Context, inputs []string) ([][]float32, error) {
	if strings.TrimSpace(s.cfg.OpenAIAPIKey) == "" {
		return nil, errors.New("OpenAI API key is not configured.")
	}
	model := strings.TrimSpace(s.cfg.OpenAIEmbeddingModel)
	if model == "" {
		return nil, errors.New("OpenAI embedding model is not configured.")
	}
	cleaned := make([]string, 0, len(inputs))
	for _, input := range inputs {
		candidate := strings.TrimSpace(input)
		if candidate == "" {
			return nil, errors.New("embedding input cannot be empty")
		}
		cleaned = append(cleaned, candidate)
	}
	if len(cleaned) == 0 {
		return nil, errors.New("embedding input cannot be empty")
	}

	reqBody := openAIEmbeddingRequest{
		Model: model,
		Input: cleaned,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAI embedding request")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAI embedding request")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.cfg.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach OpenAI embeddings")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAI embedding response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if msg := parseOpenAIErrorMessage(body); msg != "" {
			return nil, fmt.Errorf("OpenAI embedding request failed (%d): %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("OpenAI embedding request failed (%d)", resp.StatusCode)
	}

	var parsed openAIEmbeddingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI embedding response")
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("OpenAI embedding error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) != len(cleaned) {
		return nil, errors.New("OpenAI embedding response count mismatch")
	}

	out := make([][]float32, len(cleaned))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(cleaned) {
			return nil, errors.New("OpenAI embedding response index out of range")
		}
		if len(item.Embedding) != promptEmbeddingDimensions {
			return nil, fmt.Errorf("unexpected embedding size: got %d, expected %d", len(item.Embedding), promptEmbeddingDimensions)
		}
		out[item.Index] = item.Embedding
	}
	for i := range out {
		if len(out[i]) != promptEmbeddingDimensions {
			return nil, errors.New("missing embedding in OpenAI response")
		}
	}
	return out, nil
}

func embeddingVectorLiteral(embedding []float32) string {
	parts := make([]string, 0, len(embedding))
	for _, value := range embedding {
		parts = append(parts, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func cosineDistance(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1
	}
	var dot float64
	var normA float64
	var normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 1
	}
	return 1 - (dot / (sqrt(normA) * sqrt(normB)))
}

func sqrt(value float64) float64 {
	// Newton's method keeps this package dependency-free.
	if value <= 0 {
		return 0
	}
	guess := value
	for i := 0; i < 12; i++ {
		guess = 0.5 * (guess + value/guess)
	}
	return guess
}
