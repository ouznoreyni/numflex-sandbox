package main

import (
	"context"
	"log"

	"github.com/yas/numflex-sandbox/internal/config"
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

	ctx := context.Background()
	db, err := store.Open(ctx, c.DatabaseURL)
	if err != nil {
		log.Fatalf("ouverture de la base : %v", err)
	}
	defer db.Close()

	if err := seed.Run(ctx, db); err != nil {
		log.Fatalf("seed : %v", err)
	}
}
