package server

import "testing"

func TestEmbeddingVectorLiteral(t *testing.T) {
	got := embeddingVectorLiteral([]float32{1.25, -2, 0.5})
	if got != "[1.25,-2,0.5]" {
		t.Fatalf("unexpected vector literal: %q", got)
	}
}

func TestCosineDistance(t *testing.T) {
	identical := cosineDistance([]float32{1, 0, 0}, []float32{1, 0, 0})
	if identical > 0.000001 {
		t.Fatalf("expected near-zero distance for identical vectors, got %f", identical)
	}

	opposite := cosineDistance([]float32{1, 0}, []float32{-1, 0})
	if opposite < 1.99 || opposite > 2.01 {
		t.Fatalf("expected near-2 distance for opposite vectors, got %f", opposite)
	}

	mismatch := cosineDistance([]float32{1}, []float32{1, 2})
	if mismatch != 1 {
		t.Fatalf("expected 1 distance for mismatched vector lengths, got %f", mismatch)
	}
}
