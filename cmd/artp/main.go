// Package main is the regulator CLI: it carries the acts that the ARTP
// contract places outside the API gateway — validating and rejecting a
// reverse request (§6 of the guide), both reserved to the ARTP. This binary
// opens no HTTP server: it opens the pool, performs one act, and exits.
//
//	artp reverse list                lists reverse requests and their status
//	artp reverse validate <id>       validates — creates the REVERSE Request at CONFIRMATION
//	artp reverse reject <id>         rejects
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
		fmt.Fprintln(os.Stderr, "artp: "+err.Error())
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
		return fmt.Errorf("configuration: %w", err)
	}

	ctx := context.Background()
	db, err := persistence.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening the database: %w", err)
	}
	defer db.Close()

	switch args[0] {
	case "reverse":
		return executeReverse(ctx, db, args[1:])
	case "seed":
		if err := seed.Run(ctx, db, seed.VolumesFor(cfg.PoolPerOperator)); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		fmt.Println("seed replayed successfully")
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
	case "list":
		return listReverses(ctx, db)
	case "validate":
		id, err := argID(args)
		if err != nil {
			return err
		}
		if err := engine.ValidateReverse(ctx, db, id); err != nil {
			return fmt.Errorf("validating reverse request %s: %w", id, err)
		}
		fmt.Printf("reverse request %s validated: REVERSE request created at CONFIRMATION\n", id)
		return nil
	case "reject":
		id, err := argID(args)
		if err != nil {
			return err
		}
		if err := engine.RejectReverse(ctx, db, id); err != nil {
			return fmt.Errorf("rejecting reverse request %s: %w", id, err)
		}
		fmt.Printf("reverse request %s rejected\n", id)
		return nil
	default:
		return usage()
	}
}

func argID(args []string) (string, error) {
	if len(args) < 2 || args[1] == "" {
		return "", fmt.Errorf("missing reverse request identifier")
	}
	return args[1], nil
}

func listReverses(ctx context.Context, db *persistence.DB) error {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, numero, operateur_id, statut, date_demande
		   FROM reverse_request ORDER BY date_demande`)
	if err != nil {
		return fmt.Errorf("reading the reverse requests: %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var id, number, operatorID, status string
		var requestDate time.Time
		if err := rows.Scan(&id, &number, &operatorID, &status, &requestDate); err != nil {
			return fmt.Errorf("reading the reverse requests: %w", err)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", id, number, operatorID, status,
			requestDate.Format(time.RFC3339))
		n++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading the reverse requests: %w", err)
	}
	if n == 0 {
		fmt.Println("no reverse request")
	}
	return nil
}

func usage() error {
	return fmt.Errorf(`unknown command — usage:
  artp reverse list
  artp reverse validate <id>
  artp reverse reject <id>
  artp seed`)
}
