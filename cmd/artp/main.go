// Package main est le CLI régulateur : il porte les actes que le contrat
// ARTP place hors de l'API gateway — la validation et le rejet d'une demande
// de reverse (§6 du guide), tous deux réservés à l'ARTP. Ce binaire n'ouvre
// aucun serveur HTTP : il ouvre le pool, effectue un acte, et quitte.
//
//	artp reverse lister              liste les demandes de reverse et leur statut
//	artp reverse valider <id>        valide — crée la Demande REVERSE à CONFIRMATION
//	artp reverse rejeter <id>        rejette
//	artp seed                        rejoue le seed (idempotent)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/engine"
	"github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

func main() {
	if err := executer(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "artp : "+err.Error())
		os.Exit(1)
	}
}

func executer(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration : %w", err)
	}

	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("ouverture de la base : %w", err)
	}
	defer db.Close()

	switch args[0] {
	case "reverse":
		return executerReverse(ctx, db, args[1:])
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

func executerReverse(ctx context.Context, db *store.DB, args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "lister":
		return listerReverses(ctx, db)
	case "valider":
		id, err := argID(args)
		if err != nil {
			return err
		}
		if err := engine.ValiderReverse(ctx, db, id); err != nil {
			return fmt.Errorf("validation de la demande de reverse %s : %w", id, err)
		}
		fmt.Printf("demande de reverse %s validée : Demande REVERSE créée à CONFIRMATION\n", id)
		return nil
	case "rejeter":
		id, err := argID(args)
		if err != nil {
			return err
		}
		if err := engine.RejeterReverse(ctx, db, id); err != nil {
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

func listerReverses(ctx context.Context, db *store.DB) error {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, numero, operateur_id, statut, date_demande
		   FROM reverse_request ORDER BY date_demande`)
	if err != nil {
		return fmt.Errorf("lecture des demandes de reverse : %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var id, numero, operateurID, statut string
		var dateDemande time.Time
		if err := rows.Scan(&id, &numero, &operateurID, &statut, &dateDemande); err != nil {
			return fmt.Errorf("lecture des demandes de reverse : %w", err)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", id, numero, operateurID, statut,
			dateDemande.Format(time.RFC3339))
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
