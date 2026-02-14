package server

import (
	"errors"
	"testing"

	"github.com/jackc/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatalf("expected nil error to not be unique violation")
	}

	uniqueErr := &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(uniqueErr) {
		t.Fatalf("expected pg unique violation error to be recognized")
	}

	otherErr := &pgconn.PgError{Code: "22001"}
	if isUniqueViolation(otherErr) {
		t.Fatalf("expected non-unique pg error to be rejected")
	}

	wrapped := errors.New("plain error")
	if isUniqueViolation(wrapped) {
		t.Fatalf("expected generic error to be rejected")
	}
}
