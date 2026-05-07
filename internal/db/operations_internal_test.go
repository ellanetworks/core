// Copyright 2026 Ella Networks

package db

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNarrowResult_TypedAny(t *testing.T) {
	got, err := narrowResult[int]("op", 5)
	if err != nil {
		t.Fatalf("typed any: %v", err)
	}

	if got != 5 {
		t.Fatalf("typed any: got %d want 5", got)
	}
}

func TestNarrowResult_RawMessageDecodesToTyped(t *testing.T) {
	got, err := narrowResult[int]("op", json.RawMessage("5"))
	if err != nil {
		t.Fatalf("raw message: %v", err)
	}

	if got != 5 {
		t.Fatalf("raw message: got %d want 5", got)
	}
}

func TestNarrowResult_RawMessageDecodesToStruct(t *testing.T) {
	type result struct {
		Addr string `json:"addr"`
	}

	got, err := narrowResult[result]("op", json.RawMessage(`{"addr":"10.0.0.5"}`))
	if err != nil {
		t.Fatalf("raw struct: %v", err)
	}

	if got.Addr != "10.0.0.5" {
		t.Fatalf("raw struct: got %q want 10.0.0.5", got.Addr)
	}
}

func TestNarrowResult_NilReturnsZero(t *testing.T) {
	got, err := narrowResult[int]("op", nil)
	if err != nil {
		t.Fatalf("nil: %v", err)
	}

	if got != 0 {
		t.Fatalf("nil: got %d want 0", got)
	}
}

func TestNarrowResult_RawNullReturnsZero(t *testing.T) {
	got, err := narrowResult[int]("op", json.RawMessage("null"))
	if err != nil {
		t.Fatalf("raw null: %v", err)
	}

	if got != 0 {
		t.Fatalf("raw null: got %d want 0", got)
	}
}

func TestNarrowResult_VoidShortCircuits(t *testing.T) {
	// R = struct{}: any apply-side residue (last-insert-id, etc.) is dropped.
	// Without the short-circuit, an int64 last-insert-id would error.
	if _, err := narrowResult[struct{}]("op", int64(42)); err != nil {
		t.Fatalf("void / int64 residue: %v", err)
	}

	if _, err := narrowResult[struct{}]("op", json.RawMessage(`{"any":"shape"}`)); err != nil {
		t.Fatalf("void / raw residue: %v", err)
	}

	if _, err := narrowResult[struct{}]("op", nil); err != nil {
		t.Fatalf("void / nil: %v", err)
	}
}

func TestNarrowResult_TypeMismatchErrors(t *testing.T) {
	// A buggy applier returning the wrong type must surface as an error,
	// not a panic. Guards the call site against undeclared shape changes.
	_, err := narrowResult[int]("op", "not-an-int")
	if err == nil {
		t.Fatal("expected error on type mismatch")
	}

	if !strings.Contains(err.Error(), "unexpected result type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNarrowResult_RawMessageDecodeError(t *testing.T) {
	_, err := narrowResult[int]("op", json.RawMessage("not json"))
	if err == nil {
		t.Fatal("expected decode error")
	}

	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected json.SyntaxError, got %v", err)
	}
}
