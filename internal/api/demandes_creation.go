package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
	"github.com/yas/numflex-sandbox/internal/oid"
)

// routesCreation est complétée au fil des tâches : /demandes/entreprise en
// Task 11, /demandes/restitution en Task 12. Ne câbler ici que ce qui existe,
// sinon le paquet ne compile pas à la fin de cette tâche.
func (d *Deps) routesCreation(g *gin.RouterGroup) {
	g.POST("/demandes/particulier", d.postDemandeParticulier)
}

type clientDTO struct {
	Nom           string `json:"nom"`
	Prenom        string `json:"prenom"`
	DateNaissance string `json:"dateNaissance"`
	LieuNaissance string `json:"lieuNaissance"`
	TypePiece     string `json:"typePiece"`
	NumeroPiece   string `json:"numeroPiece"`
	RaisonSociale string `json:"raisonSociale"`
	NumRC         string `json:"numRC"`
}

type reqParticulier struct {
	Numero                  string    `json:"numero"`
	OtpCode                 string    `json:"otpCode"`
	OperateurSourceID       string    `json:"operateurSourceId"`
	OperateurDestinataireID string    `json:"operateurDestinataireId"`
	TypePortabilite         string    `json:"typePortabilite"`
	Client                  clientDTO `json:"client"`
}

func (d *Deps) postDemandeParticulier(c *gin.Context) {
	var req reqParticulier
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if champs := validerParticulier(req); len(champs) > 0 {
		d.R.Fail(c, apperr.Validation(champs...))
		return
	}

	appelant := Appelant(c)
	if req.OperateurDestinataireID != appelant.OperateurID {
		d.R.Fail(c, apperr.DemandeAccesRefuse(
			"L'opérateur connecté doit être l'opérateur destinataire de la demande."))
		return
	}
	if e := d.verifierOTP(c, req.Numero, req.OtpCode); e != nil {
		d.R.Fail(c, e)
		return
	}

	etat, err := d.etatNumero(c, req.Numero)
	if err != nil {
		d.R.Fail(c, err)
		return
	}
	if e := domain.VerifierEligibilitePortage(etat, req.OperateurSourceID,
		req.OperateurDestinataireID, domain.DelaiEntrePortages); e != nil {
		d.R.Fail(c, e)
		return
	}

	id := oid.New()
	maintenant := time.Now()

	tx, err2 := d.DB.Pool.Begin(c)
	if err2 != nil {
		d.R.Fail(c, apperr.ErreurInterne("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	var prefixeSource string
	if err := tx.QueryRow(c, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		req.OperateurSourceID).Scan(&prefixeSource); err != nil {
		d.R.Fail(c, apperr.ValidationEchouee("Opérateur source inconnu"))
		return
	}

	if _, err := tx.Exec(c,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,$2,'PARTICULIER','PORTAGE','EN_COURS','ACCEPTATION','EN_COURS',
		         $3,$4,$4,$5,$6,$7,$7)`,
		id, req.Numero, req.OperateurSourceID, req.OperateurDestinataireID,
		req.TypePortabilite, prefixeSource, maintenant); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("création de la demande"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_numero (demande_id, numero, statut, routage_info)
		 VALUES ($1,$2,'EN_COURS',$3)`, id, req.Numero, prefixeSource); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement du numéro"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_client
		   (demande_id, nom, prenom, date_naissance, lieu_naissance, type_piece, numero_piece)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, req.Client.Nom, req.Client.Prenom, req.Client.DateNaissance,
		req.Client.LieuNaissance, req.Client.TypePiece, req.Client.NumeroPiece); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement du client"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("validation de la transaction"))
		return
	}

	if err := d.consommerOTP(c, req.Numero); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("consommation de l'OTP"))
		return
	}

	dto, err3 := d.demandeDTO(c, id)
	if err3 != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusCreated, "Demande créée avec succès", dto)
}

// validerParticulier reproduit la validation de la plateforme, y compris son écart
// au guide : lieuNaissance est documenté facultatif mais rejeté si absent (ANO-010).
func validerParticulier(r reqParticulier) []apperr.FieldError {
	var champs []apperr.FieldError
	obligatoire := func(champ, valeur string) {
		if valeur == "" {
			champs = append(champs, apperr.FieldError{
				ObjectName: "demandeParticulierDTO", Field: champ,
				Message: "ne doit pas être vide",
			})
		}
	}
	if !motifMSISDN.MatchString(r.Numero) {
		champs = append(champs, apperr.FieldError{
			ObjectName: "demandeParticulierDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		})
	}
	obligatoire("otpCode", r.OtpCode)
	obligatoire("operateurSourceId", r.OperateurSourceID)
	obligatoire("operateurDestinataireId", r.OperateurDestinataireID)
	obligatoire("client.nom", r.Client.Nom)
	obligatoire("client.prenom", r.Client.Prenom)
	obligatoire("client.dateNaissance", r.Client.DateNaissance)
	obligatoire("client.lieuNaissance", r.Client.LieuNaissance)
	obligatoire("client.typePiece", r.Client.TypePiece)
	obligatoire("client.numeroPiece", r.Client.NumeroPiece)
	if r.TypePortabilite != "PREPAID" && r.TypePortabilite != "POSTPAID" {
		champs = append(champs, apperr.FieldError{
			ObjectName: "demandeParticulierDTO", Field: "typePortabilite",
			Message: "doit valoir PREPAID ou POSTPAID",
		})
	}
	return champs
}
