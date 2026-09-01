package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
)

// routesIncidents câble les six routes du §7.12 : deux familles (gateway,
// interne) partageant la même logique paramétrée par figeSysteme, la seule
// dimension où elles divergent réellement étant la règle "un seul incident
// interne ouvert par opérateur" (segment interne uniquement).
func (d *Deps) routesIncidents(g *gin.RouterGroup) {
	g.POST("/incidents/gateway", d.declarerIncident("gateway", false))
	g.POST("/incidents/interne", d.declarerIncident("interne", true))
	g.POST("/incidents/gateway/:id/resoudre", d.resoudreIncident("gateway"))
	g.POST("/incidents/interne/:id/resoudre", d.resoudreIncident("interne"))
	g.GET("/incidents/gateway/mes-incidents", d.mesIncidents("gateway"))
	g.GET("/incidents/interne/mes-incidents", d.mesIncidents("interne"))
}

type reqIncident struct {
	Commentaire string `json:"commentaire"`
	// typeIncidentId n'est volontairement pas décodé : le corps ne porte que
	// commentaire (§7.12). Un typeIncidentId envoyé doit être ignoré — c'est
	// le segment de l'URL qui décide de la catégorie, jamais le corps.
}

// declarerIncident construit le handler POST /incidents/{segment} pour un
// segment donné. figeSysteme fixe la catégorie attendue de type_incident :
// c'est l'endpoint qui la décide, jamais le corps de la requête.
func (d *Deps) declarerIncident(segment string, figeSysteme bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req reqIncident
		if err := c.ShouldBindJSON(&req); err != nil {
			d.R.Fail(c, entity.InvalidJSONFormat())
			return
		}

		var typeID, typeLibelle string
		err := d.DB.Pool.QueryRow(c,
			`SELECT id, libelle FROM type_incident WHERE fige_systeme = $1 LIMIT 1`,
			figeSysteme).Scan(&typeID, &typeLibelle)
		if err != nil {
			d.R.Fail(c, entity.InternalError("résolution du type d'incident"))
			return
		}

		appelant := Appelant(c)

		// Règle explicite du §7.12, réservée au segment interne : un incident
		// EN_COURS déjà ouvert chez l'appelant. L'index unique partiel de la
		// migration (incident_interne_unique_ouvert) est le garant réel côté
		// base ; ce contrôle ne fait qu'anticiper un message métier propre
		// avant la course éventuelle sur l'insertion ci-dessous.
		if figeSysteme {
			var dejaOuvert bool
			if err := d.DB.Pool.QueryRow(c,
				`SELECT EXISTS (SELECT 1 FROM incident
				   WHERE operateur_id = $1 AND statut = 'EN_COURS' AND fige_systeme)`,
				appelant.OperatorID).Scan(&dejaOuvert); err != nil {
				d.R.Fail(c, entity.InternalError("vérification des incidents ouverts"))
				return
			}
			if dejaOuvert {
				d.R.Fail(c, entity.InvalidStep(
					"Un incident interne est déjà ouvert pour votre opérateur."))
				return
			}
		}

		id := identifier.New()
		maintenant := time.Now()
		_, err = d.DB.Pool.Exec(c,
			`INSERT INTO incident
			   (id, operateur_id, type_incident_id, fige_systeme, description, statut, date_ouverture)
			 VALUES ($1,$2,$3,$4,$5,'EN_COURS',$6)`,
			id, appelant.OperatorID, typeID, figeSysteme, req.Commentaire, maintenant)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				d.R.Fail(c, entity.InvalidStep(
					"Un incident interne est déjà ouvert pour votre opérateur."))
				return
			}
			d.R.Fail(c, entity.InternalError("déclaration de l'incident"))
			return
		}

		dto, errDTO := d.incidentDTO(c, id)
		if errDTO != nil {
			d.R.Fail(c, entity.InternalError("relecture de l'incident"))
			return
		}
		d.R.OK(c, http.StatusCreated, "Incident déclaré avec succès", dto)
	}
}

// chemin de résolution attendu pour un segment donné, utilisé pour composer
// le message d'erreur du mauvais segment.
func cheminResolution(segment string) string {
	return fmt.Sprintf("%s/incidents/%s/{id}/resoudre", prefixeGateway, segment)
}

// resoudreIncident construit le handler POST /incidents/{segment}/:id/resoudre.
func (d *Deps) resoudreIncident(segment string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req reqIncident
		if err := c.ShouldBindJSON(&req); err != nil {
			d.R.Fail(c, entity.InvalidJSONFormat())
			return
		}

		var operateurID string
		var figeSysteme bool
		err := d.DB.Pool.QueryRow(c,
			`SELECT operateur_id, fige_systeme FROM incident WHERE id = $1`, id).
			Scan(&operateurID, &figeSysteme)
		if errors.Is(err, pgx.ErrNoRows) {
			// [HYP] Le guide ne fixe pas le message d'un incident inexistant.
			// Décision 4 de la tâche : réutiliser le Kind du catalogue demande,
			// mais avec le libellé incident — entity.RequestNotFound() ne
			// peut pas être réutilisée telle quelle, son message étant figé.
			d.R.Fail(c, entity.New(entity.FaultNotFound, "DEMANDE_NON_TROUVEE", "Incident introuvable"))
			return
		}
		if err != nil {
			d.R.Fail(c, entity.InternalError("lecture de l'incident"))
			return
		}

		segmentAttendu := "gateway"
		if figeSysteme {
			segmentAttendu = "interne"
		}
		if segmentAttendu != segment {
			// §7.12 : « renvoie une erreur VALIDATION_ECHOUEE indiquant le bon
			// endpoint ». L'indication doit atteindre le client — portée par un
			// fieldError, elle serait perdue, l'enveloppe de contrat (§8) ne
			// transportant que success, code, message et data.
			d.R.Fail(c, entity.ValidationFailed(
				"Cet incident se résout via POST "+cheminResolution(segmentAttendu)+"."))
			return
		}

		appelant := Appelant(c)
		if operateurID != appelant.OperatorID {
			d.R.Fail(c, entity.RequestAccessDenied(
				"Seul l'opérateur ayant déclaré l'incident peut le résoudre."))
			return
		}

		_, err = d.DB.Pool.Exec(c,
			`UPDATE incident SET statut = 'RESOLU', date_resolution = $2, commentaire_resolution = $3
			  WHERE id = $1`, id, time.Now(), req.Commentaire)
		if err != nil {
			d.R.Fail(c, entity.InternalError("résolution de l'incident"))
			return
		}

		dto, errDTO := d.incidentDTO(c, id)
		if errDTO != nil {
			d.R.Fail(c, entity.InternalError("relecture de l'incident"))
			return
		}
		d.R.OK(c, http.StatusOK, "Incident résolu avec succès", dto)
	}
}

// mesIncidents construit le handler GET /incidents/{segment}/mes-incidents.
// Contrairement aux dix listes de demandes, ces deux listes acceptent la
// pagination (page, size — défauts 0 et 20) : une asymétrie mesurée par le
// guide, pas un oubli.
func (d *Deps) mesIncidents(segment string) gin.HandlerFunc {
	figeSysteme := segment == "interne"
	return func(c *gin.Context) {
		page := parseQueryInt(c, "page", 0)
		size := parseQueryInt(c, "size", 20)

		appelant := Appelant(c)
		ids, err := d.idsIncidents(c, appelant.OperatorID, figeSysteme, page, size)
		if err != nil {
			d.R.Fail(c, entity.InternalError("lecture des incidents"))
			return
		}

		out := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			dto, errDTO := d.incidentDTO(c, id)
			if errDTO != nil {
				d.R.Fail(c, entity.InternalError("lecture de l'incident"))
				return
			}
			out = append(out, dto)
		}
		d.R.OK(c, http.StatusOK, "Incidents récupérés avec succès", out)
	}
}

// parseQueryInt lit un paramètre de requête entier non négatif, avec repli
// silencieux sur la valeur par défaut si absent ou mal formé — comme un
// paramètre Spring @RequestParam(defaultValue = ...) sur un type primitif.
func parseQueryInt(c *gin.Context, nom string, defaut int) int {
	brut := c.Query(nom)
	if brut == "" {
		return defaut
	}
	v, err := strconv.Atoi(brut)
	if err != nil || v < 0 {
		return defaut
	}
	return v
}

// idsIncidents liste, dans l'ordre chronologique, les incidents de l'appelant
// pour le segment donné, tous statuts confondus, avec pagination.
func (d *Deps) idsIncidents(ctx context.Context, operateurID string, figeSysteme bool, page, size int) ([]string, error) {
	rows, err := d.DB.Pool.Query(ctx,
		`SELECT id FROM incident
		  WHERE operateur_id = $1 AND fige_systeme = $2
		  ORDER BY date_ouverture
		  LIMIT $3 OFFSET $4`, operateurID, figeSysteme, size, page*size)
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

// incidentDTO sérialise un incident au format du §7.12 : {id, typeIncidentId,
// type, figeSysteme, description, statut, dateOuverture, operateur{id,nom}}.
func (d *Deps) incidentDTO(ctx context.Context, id string) (map[string]any, error) {
	var (
		typeID, typeLibelle, description, statut string
		figeSysteme                              bool
		dateOuverture                            time.Time
		operateurID, operateurNom                string
	)
	err := d.DB.Pool.QueryRow(ctx, `
		SELECT inc.type_incident_id, ti.libelle, inc.fige_systeme, inc.description,
		       inc.statut, inc.date_ouverture, op.id, op.nom
		  FROM incident inc
		  JOIN type_incident ti ON ti.id = inc.type_incident_id
		  JOIN operateur op ON op.id = inc.operateur_id
		 WHERE inc.id = $1`, id).Scan(
		&typeID, &typeLibelle, &figeSysteme, &description,
		&statut, &dateOuverture, &operateurID, &operateurNom)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":             id,
		"typeIncidentId": typeID,
		"type":           typeLibelle,
		"figeSysteme":    figeSysteme,
		"description":    description,
		"statut":         statut,
		"dateOuverture":  d.R.Skew(dateOuverture),
		"operateur":      map[string]any{"id": operateurID, "nom": operateurNom},
	}, nil
}
