package main

import (
	"context"
	"log"
	"os"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web"
)

func main() {
	// Arguments take precedence over the environment, which takes precedence
	// over the .env file: a container can be configured indifferently via
	// `-e`, via a mounted `.env`, or via `KEY=value` arguments.
	if err := config.ApplyArguments(os.Args[1:]); err != nil {
		log.Fatalf("arguments: %v", err)
	}
	if err := config.LoadEnvFile(); err != nil {
		log.Fatalf("configuration: %v", err)
	}

	c, err := config.Load()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	log.Printf("numflex-sandbox — fidelity=%s timeout=%s port=%s",
		c.Fidelity, c.StepTimeout, c.Port)

	if err := persistence.Migrate(c.DatabaseURL); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := persistence.Open(ctx, c.DatabaseURL)
	if err != nil {
		log.Fatalf("opening the database: %v", err)
	}
	defer db.Close()

	if err := seed.Run(ctx, db); err != nil {
		log.Fatalf("seed: %v", err)
	}

	eng := engine.New(c, db)
	go eng.Run(ctx)

	// cmd/server/main.go is the composition root: config, database,
	// migrations and seed are already built above; web.Deps carries the rest
	// (fidelity, the opened database, the running engine) into
	// web.NewRouter, which builds every gateway, unit of work, interactor,
	// presenter and controller exactly once, then wires the whole route
	// table (Task 18 — internal/api is gone, this package is now the router
	// actually served).
	d := &web.Deps{Cfg: c, DB: db, Engine: eng}
	r := web.NewRouter(d)

	if err := r.Run(":" + c.Port); err != nil {
		log.Fatalf("serveur HTTP : %v", err)
	}
}
