package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultTenantSetting is the default PostgreSQL session variable name for tenant isolation
const DefaultTenantSetting = "kgb.tenant_id"

// DefaultBypassRLSSetting is the session variable used by internal background workers to bypass tenant RLS
const DefaultBypassRLSSetting = "kgb.bypass_rls"

// DBTX is the standard interface satisfied by *pgxpool.Pool, *pgxpool.Conn, and pgx.Tx.
// It is fully compatible with sqlc-generated code using pgx/v5.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type tenantContextKey struct{}
type txContextKey struct{}
type bypassRLSContextKey struct{}

// ContextWithTenantID returns a new context with the given tenant ID.
func ContextWithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// TenantIDFromContext extracts the tenant ID from the context.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(tenantContextKey{}).(string)
	return val, ok && val != ""
}

// ContextWithBypassRLS returns a new context with the bypass RLS flag set.
func ContextWithBypassRLS(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassRLSContextKey{}, true)
}

// IsBypassRLSFromContext checks if the context has the bypass RLS flag set.
func IsBypassRLSFromContext(ctx context.Context) bool {
	val, ok := ctx.Value(bypassRLSContextKey{}).(bool)
	return ok && val
}

// ContextWithTx injects an active transaction into the context.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext retrieves an active transaction from the context if present.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

// GetDBTX returns the active transaction from context if present;
// otherwise returns the provided fallback (e.g. *pgxpool.Pool).
func GetDBTX(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return fallback
}

// TenantTxOptions holds configuration for running tenant transactions.
type TenantTxOptions struct {
	SettingName string
	TxOptions   pgx.TxOptions
}

// TenantTxOption is a functional option for configuring tenant transactions.
type TenantTxOption func(*TenantTxOptions)

// WithSettingName overrides the default session variable name ("kgb.tenant_id").
func WithSettingName(name string) TenantTxOption {
	return func(o *TenantTxOptions) {
		if name != "" {
			o.SettingName = name
		}
	}
}

// WithReadOnly configures the transaction to be read-only.
func WithReadOnly() TenantTxOption {
	return func(o *TenantTxOptions) {
		o.TxOptions.AccessMode = pgx.ReadOnly
	}
}

// WithCustomTxOptions configures custom pgx.TxOptions.
func WithCustomTxOptions(opts pgx.TxOptions) TenantTxOption {
	return func(o *TenantTxOptions) {
		o.TxOptions = opts
	}
}

// WithTenantTx executes fn inside a transaction bound to a specific tenant ID.
// It executes:
//  1. tx = pool.BeginTx(ctx, opts)
//  2. SELECT set_config(settingName, tenantID, true)  -- is_local = true (transaction-scoped)
//  3. fn(tx)
//  4. tx.Commit() / tx.Rollback()
//
// The session variable is strictly scoped to the transaction, ensuring clean return to pool.
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(tx pgx.Tx) error, opts ...TenantTxOption) (err error) {
	if pool == nil {
		return errors.New("pgxpool cannot be nil")
	}
	if tenantID == "" {
		return errors.New("tenantID cannot be empty")
	}

	options := TenantTxOptions{
		SettingName: DefaultTenantSetting,
		TxOptions:   pgx.TxOptions{},
	}
	for _, opt := range opts {
		opt(&options)
	}

	tx, err := pool.BeginTx(ctx, options.TxOptions)
	if err != nil {
		return fmt.Errorf("failed to begin tenant transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw panic after rollback
		} else if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// set_config(name, value, is_local) with is_local=true guarantees the setting
	// only lasts for the current transaction and is cleared on commit/rollback.
	setQuery := "SELECT set_config($1, $2, true)"
	if _, err = tx.Exec(ctx, setQuery, options.SettingName, tenantID); err != nil {
		return fmt.Errorf("failed to set tenant context (%s=%s): %w", options.SettingName, tenantID, err)
	}

	// Inject tx and tenantID into context for downstream consumers
	ctxWithTx := ContextWithTx(ContextWithTenantID(ctx, tenantID), tx)
	_ = ctxWithTx // Available if needed by caller

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tenant transaction: %w", err)
	}

	return nil
}

// WithTenantReadOnlyTx executes fn within a read-only tenant transaction.
func WithTenantReadOnlyTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(tx pgx.Tx) error, opts ...TenantTxOption) error {
	allOpts := append([]TenantTxOption{WithReadOnly()}, opts...)
	return WithTenantTx(ctx, pool, tenantID, fn, allOpts...)
}

// WithBypassRLS executes fn inside a transaction with the bypass flag enabled.
// This is strictly intended for internal background workers (e.g. async notification dispatchers, cron jobs).
func WithBypassRLS(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	if pool == nil {
		return errors.New("pgxpool cannot be nil")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin bypass transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	setQuery := "SELECT set_config($1, 'true', true)"
	if _, err = tx.Exec(ctx, setQuery, DefaultBypassRLSSetting); err != nil {
		return fmt.Errorf("failed to set bypass context: %w", err)
	}

	ctxWithTx := ContextWithTx(ctx, tx)
	_ = ctxWithTx

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit bypass transaction: %w", err)
	}

	return nil
}

// ConfigureTenantPool configures BeforeAcquire and AfterRelease hooks on a pgxpool.Config.
// It automatically sets the PostgreSQL session variable 'kgb.tenant_id' from ctx when acquiring
// a connection, and clears it when releasing the connection back to the pool.
// This guarantees transparent tenant isolation for all queries (including sqlc queries) with zero boilerplate.
func ConfigureTenantPool(cfg *pgxpool.Config) {
	if cfg == nil {
		return
	}

	prevBeforeAcquire := cfg.BeforeAcquire
	cfg.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		if prevBeforeAcquire != nil && !prevBeforeAcquire(ctx, conn) {
			return false
		}

		if tenantID, ok := TenantIDFromContext(ctx); ok && tenantID != "" {
			if _, err := conn.Exec(ctx, "SELECT set_config($1, $2, false)", DefaultTenantSetting, tenantID); err != nil {
				return false
			}
		}

		if IsBypassRLSFromContext(ctx) {
			if _, err := conn.Exec(ctx, "SELECT set_config($1, 'true', false)", DefaultBypassRLSSetting); err != nil {
				return false
			}
		}

		return true
	}

	prevAfterRelease := cfg.AfterRelease
	cfg.AfterRelease = func(conn *pgx.Conn) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = conn.Exec(ctx, "SELECT set_config($1, '', false), set_config($2, '', false)", DefaultTenantSetting, DefaultBypassRLSSetting)

		if prevAfterRelease != nil {
			return prevAfterRelease(conn)
		}
		return true
	}
}
