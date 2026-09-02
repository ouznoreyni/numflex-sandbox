package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// UserGateway is the Postgres implementation of port.UserGateway. Its two
// methods carry the SQL that used to live in internal/api/auth.go,
// unchanged.
type UserGateway struct {
	db Querier
}

// NewUserGateway returns a gateway bound to db — a pool for ad hoc use, or a
// transaction handed out by the unit of work.
func NewUserGateway(db Querier) *UserGateway {
	return &UserGateway{db: db}
}

// ByCredentials resolves the caller behind username and password, used by
// AuthenticateInteractor at login. found is false both for an unknown
// username and for a wrong password — the caller cannot distinguish the two,
// exactly as the legacy handler's single "Bad credentials" response never did.
func (g *UserGateway) ByCredentials(ctx context.Context, username, password string) (entity.Caller, bool, error) {
	var hash string
	var caller entity.Caller
	err := g.db.QueryRow(ctx,
		`SELECT password_hash, roles FROM utilisateur WHERE username = $1`, username).
		Scan(&hash, &caller.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Caller{}, false, nil
	}
	if err != nil {
		return entity.Caller{}, false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return entity.Caller{}, false, nil
	}
	caller.Username = username
	return caller, true, nil
}

// ByUsername resolves the caller behind a token's subject, used by the
// authentication middleware on every gateway-protected request.
func (g *UserGateway) ByUsername(ctx context.Context, username string) (entity.Caller, bool, error) {
	var caller entity.Caller
	err := g.db.QueryRow(ctx,
		`SELECT u.id, u.username, o.id, o.nom
		   FROM utilisateur u JOIN operateur o ON o.id = u.operateur_id
		  WHERE u.username = $1`, username).
		Scan(&caller.UserID, &caller.Username, &caller.OperatorID, &caller.OperatorName)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Caller{}, false, nil
	}
	if err != nil {
		return entity.Caller{}, false, err
	}
	return caller, true, nil
}
