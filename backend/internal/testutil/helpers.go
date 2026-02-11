package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"treasury-tracker/internal/database"
)

func MustNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		panic(err)
	}
	return n
}

func MustFloat64(n pgtype.Numeric) float64 {
	v, err := n.Float64Value()
	if err != nil {
		panic(err)
	}
	if !v.Valid {
		panic("invalid numeric value")
	}
	return v.Float64
}

func CleanupUser(t *testing.T, ctx context.Context, queries *database.Queries, userID int32) {
	t.Helper()
	if err := queries.DeleteUser(ctx, userID); err != nil {
		t.Logf("Warning: failed to cleanup test user %d: %v", userID, err)
	}
}
