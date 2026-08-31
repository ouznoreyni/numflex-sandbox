package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
)

func (d *Deps) routesReferentiels(g *gin.RouterGroup) {
	g.GET("/operateurs", d.getOperateurs)
	g.GET("/motifs-rejet", d.getMotifsRejet)
	g.GET("/types-demande", d.getTypesDemande)
	g.GET("/processus", d.getProcessus)
	g.GET("/types-incident", d.getTypesIncident)
}

type operateurDTO struct {
	ID  string `json:"id"`
	Nom string `json:"nom"`
}

func (d *Deps) getOperateurs(c *gin.Context) {
	rows, err := d.DB.Pool.Query(c, `SELECT id, nom FROM operateur ORDER BY nom`)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des opérateurs"))
		return
	}
	defer rows.Close()

	out := []operateurDTO{}
	for rows.Next() {
		var o operateurDTO
		if err := rows.Scan(&o.ID, &o.Nom); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture des opérateurs"))
			return
		}
		out = append(out, o)
	}
	d.R.OK(c, http.StatusOK, "Opérateurs récupérés avec succès", out)
}

type motifRejetDTO struct {
	ID    string `json:"id"`
	Motif string `json:"motif"`
}

func (d *Deps) getMotifsRejet(c *gin.Context) {
	rows, err := d.DB.Pool.Query(c, `SELECT id, motif FROM motif_rejet ORDER BY motif`)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des motifs de rejet"))
		return
	}
	defer rows.Close()

	out := []motifRejetDTO{}
	for rows.Next() {
		var m motifRejetDTO
		if err := rows.Scan(&m.ID, &m.Motif); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture des motifs de rejet"))
			return
		}
		out = append(out, m)
	}
	d.R.OK(c, http.StatusOK, "Motifs de rejet récupérés avec succès", out)
}

type typeDemandeDTO struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (d *Deps) getTypesDemande(c *gin.Context) {
	rows, err := d.DB.Pool.Query(c, `SELECT id, type FROM type_demande ORDER BY type`)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des types de demande"))
		return
	}
	defer rows.Close()

	out := []typeDemandeDTO{}
	for rows.Next() {
		var t typeDemandeDTO
		if err := rows.Scan(&t.ID, &t.Type); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture des types de demande"))
			return
		}
		out = append(out, t)
	}
	d.R.OK(c, http.StatusOK, "Types de demande récupérés avec succès", out)
}

type processusDTO struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (d *Deps) getProcessus(c *gin.Context) {
	rows, err := d.DB.Pool.Query(c, `SELECT id, type FROM processus ORDER BY type`)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des processus"))
		return
	}
	defer rows.Close()

	out := []processusDTO{}
	for rows.Next() {
		var p processusDTO
		if err := rows.Scan(&p.ID, &p.Type); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture des processus"))
			return
		}
		out = append(out, p)
	}
	d.R.OK(c, http.StatusOK, "Processus récupérés avec succès", out)
}

type typeIncidentDTO struct {
	ID          string `json:"id"`
	Libelle     string `json:"libelle"`
	FigeSysteme bool   `json:"figeSysteme"`
}

func (d *Deps) getTypesIncident(c *gin.Context) {
	rows, err := d.DB.Pool.Query(c, `SELECT id, libelle, fige_systeme FROM type_incident ORDER BY libelle`)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des types d'incident"))
		return
	}
	defer rows.Close()

	out := []typeIncidentDTO{}
	for rows.Next() {
		var ti typeIncidentDTO
		if err := rows.Scan(&ti.ID, &ti.Libelle, &ti.FigeSysteme); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture des types d'incident"))
			return
		}
		out = append(out, ti)
	}
	d.R.OK(c, http.StatusOK, "Types d'incident récupérés avec succès", out)
}
