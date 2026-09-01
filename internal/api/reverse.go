package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/apperr"
	"github.com/ouznoreyni/numflex-sandbox/internal/oid"
)

// routesReverse câble le §6 du guide : soumission et consultation des
// demandes de reverse. Aucune route d'annulation — le guide l'exclut
// explicitement (« il n'existe pas d'endpoint pour annuler une demande de
// reverse »). Les actes de l'ARTP (validation, rejet, complétion) sont hors
// périmètre de cette API : ils vivent dans internal/engine, pilotés par le
// binaire artp et, en option, par le tick du moteur.
func (d *Deps) routesReverse(g *gin.RouterGroup) {
	g.POST("/reverse-requests", d.postReverseRequest)
	g.GET("/reverse-requests/mes-demandes", d.getMesReverseRequests)
}

type reqReverse struct {
	Numero string `json:"numero"`
}

// postReverseRequest soumet une demande de reverse. Seul l'opérateur source
// (opérateur d'origine du numéro) peut le faire, et le numéro doit avoir été
// porté au moins une fois — sinon il n'y a rien à reverser.
func (d *Deps) postReverseRequest(c *gin.Context) {
	var req reqReverse
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if !motifMSISDN.MatchString(req.Numero) {
		d.R.Fail(c, apperr.Validation(apperr.FieldError{
			ObjectName: "reverseRequestDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}))
		return
	}

	etat, err := d.etatNumero(c, req.Numero)
	if err != nil {
		d.R.Fail(c, err)
		return
	}

	appelant := Appelant(c)
	if etat.OperateurOrigineID != appelant.OperateurID {
		d.R.Fail(c, apperr.DemandeAccesRefuse(
			"Seul l'opérateur source (opérateur d'origine du numéro) peut soumettre "+
				"une demande de reverse pour ce numéro."))
		return
	}
	if etat.OperateurActuelID == etat.OperateurOrigineID {
		d.R.Fail(c, apperr.NumeroNonPorte())
		return
	}

	id := oid.New()
	maintenant := time.Now()
	if _, err := d.DB.Pool.Exec(c,
		`INSERT INTO reverse_request (id, numero, operateur_id, statut, date_demande)
		 VALUES ($1,$2,$3,'EN_ATTENTE',$4)`,
		id, req.Numero, appelant.OperateurID, maintenant); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("création de la demande de reverse"))
		return
	}

	dto, errDTO := d.reverseRequestDTO(c, id)
	if errDTO != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande de reverse"))
		return
	}
	d.R.OK(c, http.StatusCreated, "Demande de reverse soumise avec succès", dto)
}

// getMesReverseRequests liste les demandes de reverse de l'appelant, tous
// statuts confondus. Contrairement aux dix files de demande, elle accepte la
// pagination (page, size — défauts 0 et 20), comme les listes d'incidents.
func (d *Deps) getMesReverseRequests(c *gin.Context) {
	page := parseQueryInt(c, "page", 0)
	size := parseQueryInt(c, "size", 20)

	appelant := Appelant(c)
	ids, err := d.idsReverseRequests(c, appelant.OperateurID, page, size)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes de reverse"))
		return
	}

	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		dto, errDTO := d.reverseRequestDTO(c, id)
		if errDTO != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture de la demande de reverse"))
			return
		}
		out = append(out, dto)
	}
	d.R.OK(c, http.StatusOK, "Demandes de reverse récupérées avec succès", out)
}

func (d *Deps) idsReverseRequests(ctx context.Context, operateurID string, page, size int) ([]string, error) {
	rows, err := d.DB.Pool.Query(ctx,
		`SELECT id FROM reverse_request
		  WHERE operateur_id = $1
		  ORDER BY date_demande
		  LIMIT $2 OFFSET $3`, operateurID, size, page*size)
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

// reverseRequestDTO sérialise une demande de reverse au format du §6 du guide :
// {id, numero, statut, dateDemande, operateur{id,nom}}.
func (d *Deps) reverseRequestDTO(ctx context.Context, id string) (map[string]any, error) {
	var numero, statut, operateurID, operateurNom string
	var dateDemande time.Time
	err := d.DB.Pool.QueryRow(ctx, `
		SELECT rr.numero, rr.statut, rr.date_demande, op.id, op.nom
		  FROM reverse_request rr
		  JOIN operateur op ON op.id = rr.operateur_id
		 WHERE rr.id = $1`, id).
		Scan(&numero, &statut, &dateDemande, &operateurID, &operateurNom)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":          id,
		"numero":      numero,
		"statut":      statut,
		"dateDemande": d.R.Skew(dateDemande),
		"operateur":   map[string]any{"id": operateurID, "nom": operateurNom},
	}, nil
}
