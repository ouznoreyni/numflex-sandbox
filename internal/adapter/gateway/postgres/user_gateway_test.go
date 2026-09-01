//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
)

// TestUserGatewayByCredentialsResolvesSeededAccount pins the two SQL
// statements moved verbatim from internal/api/auth.go: a seeded account's
// username and its real password resolve, and its roles come back exactly
// as seeded.
func TestUserGatewayByCredentialsResolvesSeededAccount(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewUserGateway(db.Pool)

	caller, found, err := g.ByCredentials(context.Background(), "yas", "yas2026")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected the seeded yas account to resolve")
	}
	if caller.Username != "yas" {
		t.Fatalf("username = %q, want yas", caller.Username)
	}
	if len(caller.Roles) != 2 || caller.Roles[0] != "ROLE_OPERATEUR_ADMIN" || caller.Roles[1] != "ROLE_USER" {
		t.Fatalf("roles = %v, want [ROLE_OPERATEUR_ADMIN ROLE_USER]", caller.Roles)
	}
}

// TestUserGatewayByCredentialsRejectsWrongPassword: found is false, not an
// error — an interactor built on this gateway distinguishes "wrong
// password" from "database unreachable" by the error return alone.
func TestUserGatewayByCredentialsRejectsWrongPassword(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewUserGateway(db.Pool)

	_, found, err := g.ByCredentials(context.Background(), "yas", "faux")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected a wrong password not to resolve")
	}
}

// TestUserGatewayByCredentialsRejectsUnknownUser mirrors the wrong-password
// case for a username that was never seeded.
func TestUserGatewayByCredentialsRejectsUnknownUser(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewUserGateway(db.Pool)

	_, found, err := g.ByCredentials(context.Background(), "fantome", "peu importe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected an unknown username not to resolve")
	}
}

// TestUserGatewayByUsernameJoinsOperateur pins the join: OperatorID and
// OperatorName come from the operateur row referenced by the seeded
// account, not from utilisateur itself.
func TestUserGatewayByUsernameJoinsOperateur(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewUserGateway(db.Pool)

	caller, found, err := g.ByUsername(context.Background(), "yas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected the seeded yas account to resolve")
	}
	if caller.OperatorID != seed.OperateurYAS {
		t.Fatalf("operatorId = %q, want %q", caller.OperatorID, seed.OperateurYAS)
	}
	if caller.OperatorName != "YAS" {
		t.Fatalf("operatorName = %q, want YAS", caller.OperatorName)
	}
	if caller.Username != "yas" {
		t.Fatalf("username = %q, want yas", caller.Username)
	}
	if caller.UserID == "" {
		t.Fatal("expected a non-empty userId")
	}
}

// TestUserGatewayByUsernameUnknownUser: found is false, no error, for a
// username that resolves neither by ByCredentials nor by ByUsername.
func TestUserGatewayByUsernameUnknownUser(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewUserGateway(db.Pool)

	_, found, err := g.ByUsername(context.Background(), "fantome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected an unknown username not to resolve")
	}
}
