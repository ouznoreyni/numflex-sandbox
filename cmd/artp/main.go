// Package main is the regulator CLI: it carries the acts that the ARTP
// contract places outside the API gateway — validating and rejecting a
// reverse request (§6 of the guide), both reserved to the ARTP. This binary
// opens no HTTP server: it opens the pool, performs one act, and exits.
//
//	artp reverse lister              lists reverse requests and their status
//	artp reverse valider <id>        validates — creates the REVERSE Request at CONFIRMATION
//	artp reverse rejeter <id>        rejects
//	artp seed                        replays the seed (idempotent)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
)

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "artp : "+err.Error())
		os.Exit(1)
	}
}

func execute(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	// No KEY=value arguments here: they would collide with the
	// subcommands. The .env file — or ENV_FILE — is still read.
	if err := config.LoadEnvFile(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration : %w", err)
	}

	ctx := context.Background()
	db, err := persistence.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("ouverture de la base : %w", err)
	}
	defer db.Close()

	switch args[0] {
	case "reverse":
		return executeReverse(ctx, db, args[1:])
	case "seed":
		if err := seed.Run(ctx, db); err != nil {
			return fmt.Errorf("seed : %w", err)
		}
		fmt.Println("seed rejoué avec succès")
		return nil
	default:
		return usage()
	}
}

func executeReverse(ctx context.Context, db *persistence.DB, args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "lister":
		return listReverses(ctx, db)
	case "valider":
		id, err := argID(args)
		if err != nil {
			return err
		}
		if err := engine.ValidateReverse(ctx, db, id); err != nil {
			return fmt.Errorf("validation de la demande de reverse %s : %w", id, err)
		}
		fmt.Printf("demande de reverse %s validée : Demande REVERSE créée à CONFIRMATION\n", id)
		return nil
	case "rejeter":
		id, err := argID(args)
		if err != nil {
			return err
		}
		if err := engine.RejectReverse(ctx, db, id); err != nil {
			return fmt.Errorf("rejet de la demande de reverse %s : %w", id, err)
		}
		fmt.Printf("demande de reverse %s rejetée\n", id)
		return nil
	default:
		return usage()
	}
}

func argID(args []string) (string, error) {
	if len(args) < 2 || args[1] == "" {
		return "", fmt.Errorf("identifiant de la demande de reverse manquant")
	}
	return args[1], nil
}

func listReverses(ctx context.Context, db *persistence.DB) error {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, numero, operateur_id, statut, date_demande
		   FROM reverse_request ORDER BY date_demande`)
	if err != nil {
		return fmt.Errorf("lecture des demandes de reverse : %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var id, number, operatorID, status string
		var requestDate time.Time
		if err := rows.Scan(&id, &number, &operatorID, &status, &requestDate); err != nil {
			return fmt.Errorf("lecture des demandes de reverse : %w", err)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", id, number, operatorID, status,
			requestDate.Format(time.RFC3339))
		n++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("lecture des demandes de reverse : %w", err)
	}
	if n == 0 {
		fmt.Println("aucune demande de reverse")
	}
	return nil
}

func usage() error {
	return fmt.Errorf(`commande inconnue — usage :
  artp reverse lister
  artp reverse valider <id>
  artp reverse rejeter <id>
  artp seed`)
}
