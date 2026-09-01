package main

import (
	"context"
	"log"
	"os"

	"github.com/ouznoreyni/numflex-sandbox/internal/api"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/httpx"
)

func main() {
	// Les arguments l'emportent sur l'environnement, qui l'emporte sur le
	// fichier .env : un conteneur se règle indifféremment par `-e`, par un
	// `.env` monté, ou par des arguments `CLEF=valeur`.
	if err := config.ApplyArguments(os.Args[1:]); err != nil {
		log.Fatalf("arguments : %v", err)
	}
	if err := config.LoadEnvFile(); err != nil {
		log.Fatalf("configuration : %v", err)
	}

	c, err := config.Load()
	if err != nil {
		log.Fatalf("configuration : %v", err)
	}
	log.Printf("numflex-sandbox — fidélité=%s expiration=%s port=%s",
		c.Fidelity, c.EtapeTimeout, c.Port)

	if err := persistence.Migrate(c.DatabaseURL); err != nil {
		log.Fatalf("migrations : %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := persistence.Open(ctx, c.DatabaseURL)
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
