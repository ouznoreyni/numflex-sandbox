package main

import (
	"context"
	"log"

	"github.com/yas/numflex-sandbox/internal/api"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/engine"
	"github.com/yas/numflex-sandbox/internal/httpx"
	"github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatalf("configuration : %v", err)
	}
	log.Printf("numflex-sandbox — fidélité=%s expiration=%s port=%s",
		c.Fidelity, c.EtapeTimeout, c.Port)

	if err := store.Migrate(c.DatabaseURL); err != nil {
		log.Fatalf("migrations : %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := store.Open(ctx, c.DatabaseURL)
	if err != nil {
		log.Fatalf("ouverture de la base : %v", err)
	}
	defer db.Close()

	if err := seed.Run(ctx, db); err != nil {
		log.Fatalf("seed : %v", err)
	}

	moteur := engine.New(c, db)
	go moteur.Run(ctx)

	d := &api.Deps{Cfg: c, DB: db, R: httpx.NewRenderer(c.Fidelity, c.ClockSkew), Moteur: moteur}
	r := api.NewRouter(d)

	if err := r.Run(":" + c.Port); err != nil {
		log.Fatalf("serveur HTTP : %v", err)
	}
}
