package server

import (
	"errors"
	"testing"

	"github.com/jackc/pgconn"
	pgconnv5 "github.com/jackc/pgx/v5/pgconn"
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

func TestIsUniqueViolationPGXV5(t *testing.T) {
	if !isUniqueViolation(&pgconnv5.PgError{Code: "23505"}) {
		t.Fatal("expected pgx/v5 unique violation to be detected")
	}
	if isUniqueViolation(&pgconnv5.PgError{Code: "22001"}) {
		t.Fatal("expected non-unique pgx/v5 error not to be detected")
	}
}
