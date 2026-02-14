package server

import (
	"context"
	"fmt"
	"strings"

	"picture-this/internal/db"
)

const (
	promptEmbeddingBackfillBatch = 100
	defaultPromptSimilarityMax   = 0.12
)

type promptSimilarityMatch struct {
	ID       uint
	Text     string
	Distance float64
}

func (s *Server) promptEmbeddingEnabled() bool {
	return strings.TrimSpace(s.cfg.OpenAIAPIKey) != "" && strings.TrimSpace(s.cfg.OpenAIEmbeddingModel) != ""
}

func (s *Server) promptSimilarityMax() float64 {
	if s.cfg.PromptSimilarityMax > 0 {
		return s.cfg.PromptSimilarityMax
	}
	return defaultPromptSimilarityMax
}

func (s *Server) ensurePromptLibraryEmbedding(ctx context.Context, promptID uint, text string) error {
	if s.db == nil || promptID == 0 || strings.TrimSpace(text) == "" || !s.promptEmbeddingEnabled() {
		return nil
	}
	embeddings, err := s.generateEmbeddingsFromOpenAI(ctx, []string{text})
	if err != nil {
		return err
	}
	return s.storePromptLibraryEmbedding(ctx, promptID, embeddings[0])
}

func (s *Server) storePromptLibraryEmbedding(ctx context.Context, promptID uint, embedding []float32) error {
	if s.db == nil || promptID == 0 || len(embedding) == 0 {
		return nil
	}
	vector := embeddingVectorLiteral(embedding)
	return s.db.WithContext(ctx).Exec(
		"UPDATE prompt_libraries SET embedding = ?::vector WHERE id = ?",
		vector,
		promptID,
	).Error
}

func (s *Server) backfillMissingPromptLibraryEmbeddings(ctx context.Context) error {
	if s.db == nil || !s.promptEmbeddingEnabled() {
		return nil
	}
	for {
		type missingPrompt struct {
			ID   uint
			Text string
		}
		missing := make([]missingPrompt, 0, promptEmbeddingBackfillBatch)
		if err := s.db.WithContext(ctx).
			Table("prompt_libraries").
			Select("id, text").
			Where("embedding IS NULL").
			Order("id ASC").
			Limit(promptEmbeddingBackfillBatch).
			Scan(&missing).Error; err != nil {
			return err
		}
		if len(missing) == 0 {
			return nil
		}

		inputs := make([]string, 0, len(missing))
		for _, entry := range missing {
			inputs = append(inputs, entry.Text)
		}
		embeddings, err := s.generateEmbeddingsFromOpenAI(ctx, inputs)
		if err != nil {
			return err
		}
		for i := range missing {
			if err := s.storePromptLibraryEmbedding(ctx, missing[i].ID, embeddings[i]); err != nil {
				return err
			}
		}
	}
}

func (s *Server) nearestPromptLibraryByEmbedding(ctx context.Context, embedding []float32, excludeID uint) (promptSimilarityMatch, bool, error) {
	if s.db == nil || len(embedding) == 0 {
		return promptSimilarityMatch{}, false, nil
	}
	vector := embeddingVectorLiteral(embedding)
	query := `
SELECT id, text, embedding <=> ?::vector AS distance
FROM prompt_libraries
WHERE embedding IS NOT NULL`
	args := []any{vector}
	if excludeID > 0 {
		query += " AND id <> ?"
		args = append(args, excludeID)
	}
	query += " ORDER BY embedding <=> ?::vector ASC LIMIT 1"
	args = append(args, vector)

	var match promptSimilarityMatch
	result := s.db.WithContext(ctx).Raw(query, args...).Scan(&match)
	if result.Error != nil {
		return promptSimilarityMatch{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return promptSimilarityMatch{}, false, nil
	}
	return match, true, nil
}

func (s *Server) filterGeneratedPromptEntries(ctx context.Context, entries []db.PromptLibrary) ([]db.PromptLibrary, map[string][]float32, error) {
	if len(entries) == 0 {
		return nil, nil, nil
	}
	if !s.promptEmbeddingEnabled() {
		return entries, nil, nil
	}
	if err := s.backfillMissingPromptLibraryEmbeddings(ctx); err != nil {
		return nil, nil, err
	}

	inputs := make([]string, 0, len(entries))
	for _, entry := range entries {
		inputs = append(inputs, entry.Text)
	}
	embeddings, err := s.generateEmbeddingsFromOpenAI(ctx, inputs)
	if err != nil {
		return nil, nil, err
	}

	threshold := s.promptSimilarityMax()
	accepted := make([]db.PromptLibrary, 0, len(entries))
	acceptedVectors := make([][]float32, 0, len(entries))
	embeddingByText := make(map[string][]float32, len(entries))
	for i, entry := range entries {
		embedding := embeddings[i]

		match, found, err := s.nearestPromptLibraryByEmbedding(ctx, embedding, 0)
		if err != nil {
			return nil, nil, err
		}
		if found && match.Distance <= threshold {
			continue
		}

		tooCloseToAccepted := false
		for _, acceptedEmbedding := range acceptedVectors {
			if cosineDistance(embedding, acceptedEmbedding) <= threshold {
				tooCloseToAccepted = true
				break
			}
		}
		if tooCloseToAccepted {
			continue
		}

		accepted = append(accepted, entry)
		acceptedVectors = append(acceptedVectors, embedding)
		embeddingByText[entry.Text] = embedding
	}
	return accepted, embeddingByText, nil
}

func (s *Server) insertPromptLibraryEntries(ctx context.Context, entries []db.PromptLibrary, embeddingsByText map[string][]float32) (int64, error) {
	if s.db == nil {
		return 0, nil
	}
	var added int64
	for _, entry := range entries {
		if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return added, err
		}
		added++
		if embedding, ok := embeddingsByText[entry.Text]; ok {
			if err := s.storePromptLibraryEmbedding(ctx, entry.ID, embedding); err != nil {
				return added, err
			}
			continue
		}
		if err := s.ensurePromptLibraryEmbedding(ctx, entry.ID, entry.Text); err != nil {
			return added, fmt.Errorf("failed to embed prompt %d: %w", entry.ID, err)
		}
	}
	return added, nil
}
