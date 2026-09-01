package seed

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/store"
)

func init() {
	store.SeedFn = func(ctx context.Context, db *store.DB) error {
		return Run(ctx, db)
	}
}
