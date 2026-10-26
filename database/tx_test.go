package database

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestWithTx_NilPool(t *testing.T) {
	ctx := context.Background()

	invoked := false
	err := WithTx(ctx, nil, func(tx pgx.Tx) error {
		if tx != nil {
			t.Fatal("expected nil tx when pool is nil")
		}
		invoked = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected fn to be invoked")
	}
}

func TestWithTx_NilPool_PropagatesError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("business logic error")

	err := WithTx(ctx, nil, func(tx pgx.Tx) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestWithTxResult_NilPool(t *testing.T) {
	ctx := context.Background()

	result, err := WithTxResult(ctx, nil, func(tx pgx.Tx) (string, error) {
		if tx != nil {
			t.Fatal("expected nil tx when pool is nil")
		}
		return "hello-result", nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != "hello-result" {
		t.Fatalf("expected 'hello-result', got %s", result)
	}
}

func TestWithTxResult_NilPool_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("calc error")

	result, err := WithTxResult(ctx, nil, func(tx pgx.Tx) (int, error) {
		return 0, expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if result != 0 {
		t.Fatalf("expected 0 result on error, got %d", result)
	}
}
