package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type TxFunc func(*sql.Tx) error

func WithTransaction(ctx context.Context, db *sql.DB, fn TxFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type Pagination struct {
	Limit  int32
	Offset int32
}

func NewPagination(page, pageSize int32) Pagination {
	if page < 1 {
		page = 1
	}
	return Pagination{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func WrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
