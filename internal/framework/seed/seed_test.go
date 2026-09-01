package seed_test

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestOperateursIdentifiantsExacts(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	attendus := map[string]string{
		"6a21745ce6c37b5b5b487ec1": "ORANGE",
		"6a2174c3e6c37b5b5b487ec4": "YAS",
		"6a217510e6c37b5b5b487ec7": "EXPRESSO",
	}
	for id, nom := range attendus {
		var got string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT nom FROM operateur WHERE id = $1", id).Scan(&got),
			"opérateur %s absent", id)
		require.Equal(t, nom, got)
	}

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM operateur").Scan(&n))
	require.Equal(t, 3, n)
}

func TestMotifsRejetIdentifiantsExacts(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	attendus := map[string]string{
		"6a2175c5e6c37b5b5b487edb": "Dernier portage inférieur à 3 mois",
		"6a2175cfe6c37b5b5b487edc": "Erreur sur les infos",
		"6a2175d9e6c37b5b5b487edd": "Données manquantes",
		"6a2175e7e6c37b5b5b487ede": "Numéro Inactif",
		"6a2175f3e6c37b5b5b487edf": "Identité non prouvée",
		"6a2175fde6c37b5b5b487ee0": "Engagement en cours dans une demande",
	}
	for id, motif := range attendus {
		var got string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT motif FROM motif_rejet WHERE id = $1", id).Scan(&got), "motif %s absent", id)
		require.Equal(t, motif, got)
	}
}

func TestComptes(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	comptes := map[string]struct {
		motDePasse string
		operateur  string
	}{
		"orange":   {"orange2026", "6a21745ce6c37b5b5b487ec1"},
		"yas":      {"yas2026", "6a2174c3e6c37b5b5b487ec4"},
		"expresso": {"expresso2026", "6a217510e6c37b5b5b487ec7"},
	}
	for username, attendu := range comptes {
		var hash, operateurID string
		var roles []string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT password_hash, operateur_id, roles FROM utilisateur WHERE username = $1",
			username).Scan(&hash, &operateurID, &roles), "compte %s absent", username)

		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(attendu.motDePasse)))
		require.Equal(t, attendu.operateur, operateurID)
		require.ElementsMatch(t, []string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"}, roles)
	}
}

func TestVivierNumeros(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	cas := []struct {
		msisdn            string
		operateurActuel   string
		portage           bool
		dejaRestitue      bool
		agePortageMinJour int
		agePortageMaxJour int
	}{
		{"771000001", seed.OperateurOrange, false, false, 0, 0},
		{"761000001", seed.OperateurYAS, false, false, 0, 0},
		{"701000001", seed.OperateurExpresso, false, false, 0, 0},
		{"772000001", seed.OperateurOrange, true, false, 25, 35},
		{"773000001", seed.OperateurYAS, true, false, 230, 250},
		{"774000001", seed.OperateurYAS, true, false, 55, 65},
		{"775000001", seed.OperateurYAS, true, true, 230, 250},
	}
	for _, c := range cas {
		var actuel string
		var date *time.Time
		var restitue bool
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			`SELECT operateur_actuel_id, date_dernier_portage, deja_restitue
			 FROM numero WHERE msisdn = $1`, c.msisdn).Scan(&actuel, &date, &restitue),
			"numéro %s absent du vivier", c.msisdn)

		require.Equal(t, c.operateurActuel, actuel, c.msisdn)
		require.Equal(t, c.dejaRestitue, restitue, c.msisdn)
		if !c.portage {
			require.Nilf(t, date, "%s ne doit pas porter de date de portage", c.msisdn)
			continue
		}
		require.NotNilf(t, date, "%s doit porter une date de portage", c.msisdn)
		age := int(time.Since(*date).Hours() / 24)
		require.GreaterOrEqual(t, age, c.agePortageMinJour, c.msisdn)
		require.LessOrEqual(t, age, c.agePortageMaxJour, c.msisdn)
	}
}

func TestSeedIdempotent(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	require.NoError(t, seed.Run(ctx, db))
	require.NoError(t, seed.Run(ctx, db))

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM operateur").Scan(&n))
	require.Equal(t, 3, n)
}
