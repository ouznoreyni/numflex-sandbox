package seed

import (
	"context"

	"github.com/yas/numflex-sandbox/internal/store"
)

func init() {
	store.SeedFn = func(ctx context.Context, db *store.DB) error {
		return Run(ctx, db)
	}
}
