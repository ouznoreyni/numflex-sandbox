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

func TestOperatorsExactIdentifiers(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	expected := map[string]string{
		"6a21745ce6c37b5b5b487ec1": "ORANGE",
		"6a2174c3e6c37b5b5b487ec4": "YAS",
		"6a217510e6c37b5b5b487ec7": "EXPRESSO",
	}
	for id, name := range expected {
		var got string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT nom FROM operateur WHERE id = $1", id).Scan(&got),
			"operator %s missing", id)
		require.Equal(t, name, got)
	}

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM operateur").Scan(&n))
	require.Equal(t, 3, n)
}

func TestRejectionReasonsExactIdentifiers(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	expected := map[string]string{
		"6a2175c5e6c37b5b5b487edb": "Dernier portage inférieur à 3 mois",
		"6a2175cfe6c37b5b5b487edc": "Erreur sur les infos",
		"6a2175d9e6c37b5b5b487edd": "Données manquantes",
		"6a2175e7e6c37b5b5b487ede": "Numéro Inactif",
		"6a2175f3e6c37b5b5b487edf": "Identité non prouvée",
		"6a2175fde6c37b5b5b487ee0": "Engagement en cours dans une demande",
	}
	for id, reason := range expected {
		var got string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT motif FROM motif_rejet WHERE id = $1", id).Scan(&got), "motif %s absent", id)
		require.Equal(t, reason, got)
	}
}

func TestAccounts(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	accounts := map[string]struct {
		password string
		operator string
	}{
		"orange":   {"orange2026", "6a21745ce6c37b5b5b487ec1"},
		"yas":      {"yas2026", "6a2174c3e6c37b5b5b487ec4"},
		"expresso": {"expresso2026", "6a217510e6c37b5b5b487ec7"},
	}
	for username, expected := range accounts {
		var hash, operatorID string
		var roles []string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT password_hash, operateur_id, roles FROM utilisateur WHERE username = $1",
			username).Scan(&hash, &operatorID, &roles), "compte %s absent", username)

		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(expected.password)))
		require.Equal(t, expected.operator, operatorID)
		require.ElementsMatch(t, []string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"}, roles)
	}
}

func TestNumberPool(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	cases := []struct {
		msisdn          string
		currentOperator string
		porting         bool
		alreadyReturned bool
		portingAgeMinD  int
		portingAgeMaxD  int
	}{
		{"771000001", seed.OperatorOrangeID, false, false, 0, 0},
		{"761000001", seed.OperatorYASID, false, false, 0, 0},
		{"701000001", seed.OperatorExpressoID, false, false, 0, 0},
		{"779000001", seed.OperatorOrangeID, true, false, 25, 35},
		{"789001001", seed.OperatorYASID, true, false, 230, 250},
		{"789002001", seed.OperatorYASID, true, false, 55, 65},
		{"789003001", seed.OperatorYASID, true, true, 230, 250},
		// Range ends: a range starts at 000000, so the thousandth number
		// of a thousand-strong range is 000999, and the last block of a
		// ported range stops at 003999.
		{"771000000", seed.OperatorOrangeID, false, false, 0, 0},
		{"771000999", seed.OperatorOrangeID, false, false, 0, 0},
		{"719003999", seed.OperatorExpressoID, true, true, 230, 250},
	}
	for _, c := range cases {
		var current string
		var date *time.Time
		var returned bool
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			`SELECT operateur_actuel_id, date_dernier_portage, deja_restitue
			 FROM numero WHERE msisdn = $1`, c.msisdn).Scan(&current, &date, &returned),
			"number %s missing from the pool", c.msisdn)

		require.Equal(t, c.currentOperator, current, c.msisdn)
		require.Equal(t, c.alreadyReturned, returned, c.msisdn)
		if !c.porting {
			require.Nilf(t, date, "%s must not carry a porting date", c.msisdn)
			continue
		}
		require.NotNilf(t, date, "%s must carry a porting date", c.msisdn)
		age := int(time.Since(*date).Hours() / 24)
		require.GreaterOrEqual(t, age, c.portingAgeMinD, c.msisdn)
		require.LessOrEqual(t, age, c.portingAgeMaxD, c.msisdn)
	}
}

func TestNumberPoolVolume(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	// 1000 per range: ORANGE 8 unported + 4 ported blocks, YAS and EXPRESSO
	// 9 unported each — their historical range included — plus 4 blocks.
	v := seed.TestVolumes
	perOperator := map[string]int{
		seed.OperatorOrangeID: seed.UnportedRangesPerOperator*v.OrangeYAS +
			seed.PortedScenarioCount*v.PortedBlock,
		seed.OperatorYASID: seed.UnportedRangesPerOperator*v.OrangeYAS + v.Historical +
			seed.PortedScenarioCount*v.PortedBlock,
		seed.OperatorExpressoID: seed.UnportedRangesPerOperator*v.Expresso + v.Historical +
			seed.PortedScenarioCount*v.PortedBlock,
	}
	total := 0
	for id, expected := range perOperator {
		var n int
		require.NoError(t, db.Pool.QueryRow(ctx,
			"SELECT count(*) FROM numero WHERE operateur_actuel_id = $1", id).Scan(&n))
		require.Equal(t, expected, n, id)
		total += expected
	}

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM numero").Scan(&n))
	require.Equal(t, total, n)

	// A range stops at its size: nothing beyond it.
	require.NoError(t, db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM numero WHERE msisdn = '771001000'").Scan(&n))
	require.Zero(t, n)
}

func TestSeedIdempotent(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	require.NoError(t, seed.Run(ctx, db, seed.TestVolumes))
	require.NoError(t, seed.Run(ctx, db, seed.TestVolumes))

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM operateur").Scan(&n))
	require.Equal(t, 3, n)
}
