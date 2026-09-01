package postgres

// SandboxGateway is the Postgres implementation of port.SandboxGateway. Its
// methods carry the SQL that used to live in the deleted
// internal/api/sandbox.go's deletePurgeDemandes, unchanged — five
// statements, always run inside the same port.UnitOfWork transaction
// (Repositories.Sandbox), so a failure anywhere among them leaves every one
// of them undone.

import "context"

type SandboxGateway struct {
	db Querier
}

// NewSandboxGateway returns a gateway bound to db — always a transaction
// handed out by the unit of work: every method here writes, or reads
// something a later write in the same purge depends on, and the whole
// operation must see one consistent snapshot.
func NewSandboxGateway(db Querier) *SandboxGateway {
	return &SandboxGateway{db: db}
}

// RequestIDsToPurge — moved verbatim from deletePurgeDemandes' first query.
// The scope is createur_operateur_id, never the /mes-demandes filter: a
// request belongs to two operators at once, and only its creator made it.
func (g *SandboxGateway) RequestIDsToPurge(ctx context.Context, operatorID string) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM demande WHERE createur_operateur_id = $1`, operatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// NumbersToRestore — moved verbatim from deletePurgeDemandes' second query.
// A particulier request's number lives on demande.numero; a fleet's on
// demande_numero. Both sources count, exclus compris: they too may have
// moved before being excluded.
func (g *SandboxGateway) NumbersToRestore(ctx context.Context, requestIDs []string) ([]string, error) {
	rows, err := g.db.Query(ctx, `
		SELECT numero FROM demande WHERE id = ANY($1)
		 UNION
		SELECT numero FROM demande_numero WHERE demande_id = ANY($1)`, requestIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	numeros := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		numeros = append(numeros, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return numeros, nil
}

// DeleteReverseRequests — moved verbatim from deletePurgeDemandes: ahead of
// DeleteRequests, since reverse_request's foreign key carries no ON DELETE
// CASCADE and would block it.
func (g *SandboxGateway) DeleteReverseRequests(ctx context.Context, operatorID string, requestIDs []string) (int64, error) {
	tag, err := g.db.Exec(ctx,
		`DELETE FROM reverse_request WHERE operateur_id = $1 OR demande_id = ANY($2)`,
		operatorID, requestIDs)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteOTP — moved verbatim from deletePurgeDemandes.
func (g *SandboxGateway) DeleteOTP(ctx context.Context, numbers []string) (int64, error) {
	tag, err := g.db.Exec(ctx, `DELETE FROM otp WHERE numero = ANY($1)`, numbers)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteRequests — moved verbatim from deletePurgeDemandes. demande_numero,
// demande_client, etape_historique and confirmation carry ON DELETE CASCADE
// and follow.
func (g *SandboxGateway) DeleteRequests(ctx context.Context, requestIDs []string) (int64, error) {
	tag, err := g.db.Exec(ctx, `DELETE FROM demande WHERE id = ANY($1)`, requestIDs)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RestoreNumbers — moved verbatim from deletePurgeDemandes. The rule is
// "le numéro rentre chez lui" (operateur_origine_id), not "le numéro
// retrouve son état de seed": a seeded number already ported before the
// purge returns to its origin operator, not to its seeded holder.
func (g *SandboxGateway) RestoreNumbers(ctx context.Context, numbers []string) (int64, error) {
	tag, err := g.db.Exec(ctx, `
		UPDATE numero
		   SET operateur_actuel_id = operateur_origine_id,
		       date_dernier_portage = NULL,
		       deja_restitue = false
		 WHERE msisdn = ANY($1)`, numbers)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
