package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestContextWithTenantID(t *testing.T) {
	ctx := context.Background()
	tenantID := "f47ac10b-58cc-4372-a567-0e02b2c3d479"

	// Initially empty
	gotID, ok := TenantIDFromContext(ctx)
	assert.False(t, ok)
	assert.Empty(t, gotID)

	// Injected
	ctx = ContextWithTenantID(ctx, tenantID)
	gotID, ok = TenantIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, tenantID, gotID)
}

func TestWithTenantTx_Validations(t *testing.T) {
	ctx := context.Background()

	// Nil pool should return error
	err := WithTenantTx(ctx, nil, "valid-uuid", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pgxpool cannot be nil")

	// Empty tenantID should return error
	err = WithTenantTx(ctx, nil, "", nil)
	assert.Error(t, err)

	// WithTenantTxContext validation with nil pool
	err = WithTenantTxContext(ctx, nil, "valid-uuid", func(c context.Context, tx pgx.Tx) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pgxpool cannot be nil")
}

func TestGetDBTX_Fallback(t *testing.T) {
	ctx := context.Background()

	// When no tx in context, fallback should be returned
	var fallback DBTX = nil
	result := GetDBTX(ctx, fallback)
	assert.Nil(t, result)
}

func TestBypassRLSContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsBypassRLSFromContext(ctx))

	ctxBypass := ContextWithBypassRLS(ctx)
	assert.True(t, IsBypassRLSFromContext(ctxBypass))
}
