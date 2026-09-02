package inmemory

import (
	"context"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// UserGateway is a map-backed port.UserGateway, keyed by username, for use
// case unit tests that must not touch a database.
type UserGateway struct {
	mu    sync.Mutex
	users map[string]seededUser
}

type seededUser struct {
	caller       entity.Caller
	passwordHash []byte
}

// NewUserGateway returns an empty double, ready to use.
func NewUserGateway() *UserGateway {
	return &UserGateway{users: make(map[string]seededUser)}
}

// Seed registers a user under username, resolvable by ByUsername as caller
// and by ByCredentials when password matches — mirroring how the real
// gateway compares a bcrypt hash rather than a plain string, so an
// interactor test cannot accidentally pass by comparing bytes.
func (g *UserGateway) Seed(t testingT, username, password string, caller entity.Caller) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.users == nil {
		g.users = make(map[string]seededUser)
	}
	g.users[username] = seededUser{caller: caller, passwordHash: hash}
}

// testingT is the narrow slice of *testing.T that Seed needs — kept narrow
// so this file avoids importing "testing" into a type other packages'
// non-test code might otherwise be tempted to depend on.
type testingT interface {
	Helper()
	Fatal(args ...any)
}

func (g *UserGateway) ByCredentials(_ context.Context, username, password string) (entity.Caller, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	u, ok := g.users[username]
	if !ok {
		return entity.Caller{}, false, nil
	}
	if bcrypt.CompareHashAndPassword(u.passwordHash, []byte(password)) != nil {
		return entity.Caller{}, false, nil
	}
	return u.caller, true, nil
}

func (g *UserGateway) ByUsername(_ context.Context, username string) (entity.Caller, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	u, ok := g.users[username]
	return u.caller, ok, nil
}
