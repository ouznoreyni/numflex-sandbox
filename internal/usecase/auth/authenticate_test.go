package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/auth"
)

var errBoom = errors.New("boom")

// TestAuthenticateRejectsUnknownUser pins that a bad login yields the
// platform's own 401 shape rather than a Go error leaking through.
func TestAuthenticateRejectsUnknownUser(t *testing.T) {
	users := inmemory.NewUserGateway()
	i := auth.NewAuthenticate(users, func(string, []string) (string, error) {
		t.Fatal("token must not be issued for an unknown user")
		return "", nil
	}, 24*time.Hour)

	_, fault := i.Execute(context.Background(), auth.AuthenticateInput{
		Username: "ghost", Password: "whatever"})
	if fault == nil {
		t.Fatal("expected a fault for an unknown user")
	}
	if fault.Kind != entity.FaultAccess {
		t.Fatalf("expected an access fault, got kind %v", fault.Kind)
	}
}

// TestAuthenticateRejectsWrongPassword: a known username with the wrong
// password must fail exactly like an unknown one — the caller cannot tell
// the two apart, mirroring the platform's single "Bad credentials" answer.
func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	users := inmemory.NewUserGateway()
	users.Seed(t, "yas", "yas2026", entity.Caller{Username: "yas"})

	i := auth.NewAuthenticate(users, func(string, []string) (string, error) {
		t.Fatal("token must not be issued for a wrong password")
		return "", nil
	}, 24*time.Hour)

	_, fault := i.Execute(context.Background(), auth.AuthenticateInput{
		Username: "yas", Password: "faux"})
	if fault == nil {
		t.Fatal("expected a fault for a wrong password")
	}
	if fault.Kind != entity.FaultAccess {
		t.Fatalf("expected an access fault, got kind %v", fault.Kind)
	}
}

// TestAuthenticateIssuesTokenForKnownUser pins the happy path: the resolved
// caller's username and roles are handed unchanged to the injected
// TokenIssuer, and its return value flows straight into the output.
func TestAuthenticateIssuesTokenForKnownUser(t *testing.T) {
	users := inmemory.NewUserGateway()
	users.Seed(t, "yas", "yas2026", entity.Caller{
		Username: "yas", Roles: []string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"},
	})

	var gotUsername string
	var gotRoles []string
	i := auth.NewAuthenticate(users, func(username string, roles []string) (string, error) {
		gotUsername, gotRoles = username, roles
		return "jeton-signe", nil
	}, 24*time.Hour)

	out, fault := i.Execute(context.Background(), auth.AuthenticateInput{
		Username: "yas", Password: "yas2026"})
	if fault != nil {
		t.Fatalf("expected success, got fault %v", fault)
	}
	if out.Token != "jeton-signe" {
		t.Fatalf("token = %q, want jeton-signe", out.Token)
	}
	if gotUsername != "yas" {
		t.Fatalf("issuer username = %q, want yas", gotUsername)
	}
	if len(gotRoles) != 2 || gotRoles[0] != "ROLE_OPERATEUR_ADMIN" || gotRoles[1] != "ROLE_USER" {
		t.Fatalf("issuer roles = %v, want [ROLE_OPERATEUR_ADMIN ROLE_USER]", gotRoles)
	}
}

// TestAuthenticatePropagatesTokenIssuerError: a signing failure must surface
// as an internal *entity.Fault, not a bare Go error.
func TestAuthenticatePropagatesTokenIssuerError(t *testing.T) {
	users := inmemory.NewUserGateway()
	users.Seed(t, "yas", "yas2026", entity.Caller{Username: "yas"})

	i := auth.NewAuthenticate(users, func(string, []string) (string, error) {
		return "", errBoom
	}, 24*time.Hour)

	_, fault := i.Execute(context.Background(), auth.AuthenticateInput{
		Username: "yas", Password: "yas2026"})
	if fault == nil {
		t.Fatal("expected a fault when the token issuer fails")
	}
	if fault.Kind != entity.FaultInternal {
		t.Fatalf("expected an internal fault, got kind %v", fault.Kind)
	}
}
