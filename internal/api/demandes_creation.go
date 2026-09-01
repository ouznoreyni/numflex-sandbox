package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/oid"
)

// routesCreation est complétée au fil des tâches : /demandes/restitution en
// Task 12. Ne câbler ici que ce qui existe, sinon le paquet ne compile pas à
// la fin de cette tâche.
func (d *Deps) routesCreation(g *gin.RouterGroup) {
	g.POST("/demandes/particulier", d.postDemandeParticulier)
	g.POST("/demandes/entreprise", d.postDemandeEntreprise)
	g.POST("/demandes/restitution", d.postDemandeRestitution)
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
		d.R.Fail(c, entity.InvalidJSONFormat())
		return
	}
	if champs := validerParticulier(req); len(champs) > 0 {
		d.R.Fail(c, entity.Validation(champs...))
		return
	}

	appelant := Appelant(c)
	if req.OperateurDestinataireID != appelant.OperatorID {
		d.R.Fail(c, entity.RequestAccessDenied(
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
	if e := entity.CheckPortingEligibility(etat, req.OperateurSourceID,
		req.OperateurDestinataireID, entity.DelayBetweenPortings); e != nil {
		d.R.Fail(c, e)
		return
	}

	id := oid.New()
	maintenant := time.Now()

	tx, err2 := d.DB.Pool.Begin(c)
	if err2 != nil {
		d.R.Fail(c, entity.InternalError("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	var prefixeSource string
	if err := tx.QueryRow(c, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		req.OperateurSourceID).Scan(&prefixeSource); err != nil {
		d.R.Fail(c, entity.ValidationFailed("Opérateur source inconnu"))
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
		d.R.Fail(c, entity.InternalError("création de la demande"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_numero (demande_id, numero, statut, routage_info)
		 VALUES ($1,$2,'EN_COURS',$3)`, id, req.Numero, prefixeSource); err != nil {
		d.R.Fail(c, entity.InternalError("enregistrement du numéro"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_client
		   (demande_id, nom, prenom, date_naissance, lieu_naissance, type_piece, numero_piece)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, req.Client.Nom, req.Client.Prenom, req.Client.DateNaissance,
		req.Client.LieuNaissance, req.Client.TypePiece, req.Client.NumeroPiece); err != nil {
		d.R.Fail(c, entity.InternalError("enregistrement du client"))
		return
	}
	if _, err := tx.Exec(c,
		`UPDATE otp SET consomme = true WHERE numero = $1`, req.Numero); err != nil {
		d.R.Fail(c, entity.InternalError("consommation de l'OTP"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, entity.InternalError("validation de la transaction"))
		return
	}

	dto, err3 := d.demandeDTO(c, id)
	if err3 != nil {
		d.R.Fail(c, entity.InternalError("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusCreated, "Demande particulier créée avec succès", dto)
}

// validerParticulier reproduit la validation de la plateforme, y compris son écart
// au guide : lieuNaissance est documenté facultatif mais rejeté si absent (ANO-010).
func validerParticulier(r reqParticulier) []entity.FieldFault {
	var champs []entity.FieldFault
	obligatoire := func(champ, valeur string) {
		if valeur == "" {
			champs = append(champs, entity.FieldFault{
				ObjectName: "demandeParticulierDTO", Field: champ,
				Message: "ne doit pas être vide",
			})
		}
	}
	if !motifMSISDN.MatchString(r.Numero) {
		champs = append(champs, entity.FieldFault{
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
		champs = append(champs, entity.FieldFault{
			ObjectName: "demandeParticulierDTO", Field: "typePortabilite",
			Message: "doit valoir PREPAID ou POSTPAID",
		})
	}
	return champs
}

// --- Entreprise (flotte) ----------------------------------------------------

type reqEntreprise struct {
	NumeroPorteurFlotte     string    `json:"numeroPorteurFlotte"`
	OtpCode                 string    `json:"otpCode"`
	OperateurSourceID       string    `json:"operateurSourceId"`
	OperateurDestinataireID string    `json:"operateurDestinataireId"`
	TypePortabilite         string    `json:"typePortabilite"`
	NumerosFlotte           []string  `json:"numerosFlotte"`
	Client                  clientDTO `json:"client"`
}

// numeroExclu porte le motif d'exclusion d'un numéro de la flotte (§7.4). Un
// numéro exclu n'échoue pas la demande : il en est simplement retranché.
type numeroExclu struct {
	Numero     string `json:"numero"`
	Raison     string `json:"raison"`
	CodeErreur string `json:"codeErreur"`
}

func (d *Deps) postDemandeEntreprise(c *gin.Context) {
	var req reqEntreprise
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, entity.InvalidJSONFormat())
		return
	}
	if champs := validerEntreprise(req); len(champs) > 0 {
		d.R.Fail(c, entity.Validation(champs...))
		return
	}
	// §9 : le catalogue réserve un code à ce cas précis. Le traiter comme une
	// violation de bean validation le rendrait inatteignable.
	if len(req.NumerosFlotte) == 0 {
		d.R.Fail(c, entity.FleetEmpty())
		return
	}

	appelant := Appelant(c)
	if req.OperateurDestinataireID != appelant.OperatorID {
		d.R.Fail(c, entity.RequestAccessDenied(
			"L'opérateur connecté doit être l'opérateur destinataire de la demande."))
		return
	}

	// Un seul OTP, vérifié sur le porteur de la flotte, couvre tous les numéros.
	if e := d.verifierOTP(c, req.NumeroPorteurFlotte, req.OtpCode); e != nil {
		d.R.Fail(c, e)
		return
	}

	etats := make(map[string]entity.NumberState, len(req.NumerosFlotte))
	for _, numero := range req.NumerosFlotte {
		etat, e := d.etatNumero(c, numero)
		if e != nil {
			d.R.Fail(c, e)
			return
		}
		etats[numero] = etat
	}
	for _, numero := range req.NumerosFlotte {
		if etats[numero].CurrentOperatorID != etats[req.NumerosFlotte[0]].CurrentOperatorID {
			d.R.Fail(c, entity.FleetMixedOperators())
			return
		}
	}

	var retenus []string
	exclus := []numeroExclu{}
	for _, numero := range req.NumerosFlotte {
		if e := entity.CheckPortingEligibility(etats[numero], req.OperateurSourceID,
			req.OperateurDestinataireID, entity.DelayBetweenPortings); e != nil {
			exclus = append(exclus, numeroExclu{Numero: numero, Raison: e.Message, CodeErreur: e.Code})
			continue
		}
		retenus = append(retenus, numero)
	}
	if len(retenus) == 0 {
		d.R.Fail(c, entity.NoEligibleNumber())
		return
	}

	id := oid.New()
	maintenant := time.Now()

	tx, err2 := d.DB.Pool.Begin(c)
	if err2 != nil {
		d.R.Fail(c, entity.InternalError("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	var prefixeSource string
	if err := tx.QueryRow(c, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		req.OperateurSourceID).Scan(&prefixeSource); err != nil {
		d.R.Fail(c, entity.ValidationFailed("Opérateur source inconnu"))
		return
	}

	if _, err := tx.Exec(c,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,$2,'ENTREPRISE','PORTAGE','EN_COURS','ACCEPTATION','EN_COURS',
		         $3,$4,$4,$5,$6,$7,$7)`,
		id, req.NumeroPorteurFlotte, req.OperateurSourceID, req.OperateurDestinataireID,
		req.TypePortabilite, prefixeSource, maintenant); err != nil {
		d.R.Fail(c, entity.InternalError("création de la demande"))
		return
	}

	for _, numero := range retenus {
		if _, err := tx.Exec(c,
			`INSERT INTO demande_numero (demande_id, numero, statut, routage_info)
			 VALUES ($1,$2,'EN_COURS',$3)`, id, numero, prefixeSource); err != nil {
			d.R.Fail(c, entity.InternalError("enregistrement du numéro"))
			return
		}
	}
	for _, ex := range exclus {
		if _, err := tx.Exec(c,
			`INSERT INTO demande_numero
			   (demande_id, numero, statut, exclu, raison_exclusion, code_erreur_exclusion)
			 VALUES ($1,$2,'REJETE',true,$3,$4)`,
			id, ex.Numero, ex.Raison, ex.CodeErreur); err != nil {
			d.R.Fail(c, entity.InternalError("enregistrement du numéro exclu"))
			return
		}
	}

	if _, err := tx.Exec(c,
		`INSERT INTO demande_client
		   (demande_id, nom, prenom, date_naissance, lieu_naissance, type_piece, numero_piece,
		    raison_sociale, num_rc)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, req.Client.Nom, req.Client.Prenom, req.Client.DateNaissance,
		req.Client.LieuNaissance, req.Client.TypePiece, req.Client.NumeroPiece,
		req.Client.RaisonSociale, req.Client.NumRC); err != nil {
		d.R.Fail(c, entity.InternalError("enregistrement du client"))
		return
	}
	if _, err := tx.Exec(c,
		`UPDATE otp SET consomme = true WHERE numero = $1`, req.NumeroPorteurFlotte); err != nil {
		d.R.Fail(c, entity.InternalError("consommation de l'OTP"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, entity.InternalError("validation de la transaction"))
		return
	}

	data := gin.H{
		"demande": gin.H{
			"id":            id,
			"typeDemande":   "PORTAGE",
			"typeAbonne":    "ENTREPRISE",
			"statutDemande": "EN_COURS",
			"etapeActuelle": "ACCEPTATION",
		},
		"numerosPortesCount": len(retenus),
		"numerosExclusCount": len(exclus),
		"numerosExclus":      exclus,
	}
	if len(exclus) > 0 {
		data["avertissement"] = fmt.Sprintf("%d numéro(s) exclu(s) de la demande.", len(exclus))
	}
	d.R.OK(c, http.StatusCreated, "Demande flotte créée", data)
}

// validerEntreprise reproduit la validation de forme d'une demande flotte : les
// mêmes champs obligatoires qu'une demande particulier, mais numeroPorteurFlotte
// à la place de numero, raisonSociale/numRC à la place d'une identité seule, et
// numerosFlotte dont la vacuité relève du code FLOTTE_VIDE, hors de cette fonction.
func validerEntreprise(r reqEntreprise) []entity.FieldFault {
	var champs []entity.FieldFault
	obligatoire := func(champ, valeur string) {
		if valeur == "" {
			champs = append(champs, entity.FieldFault{
				ObjectName: "demandeEntrepriseDTO", Field: champ,
				Message: "ne doit pas être vide",
			})
		}
	}
	if !motifMSISDN.MatchString(r.NumeroPorteurFlotte) {
		champs = append(champs, entity.FieldFault{
			ObjectName: "demandeEntrepriseDTO", Field: "numeroPorteurFlotte",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		})
	}
	obligatoire("otpCode", r.OtpCode)
	obligatoire("operateurSourceId", r.OperateurSourceID)
	obligatoire("operateurDestinataireId", r.OperateurDestinataireID)
	obligatoire("client.raisonSociale", r.Client.RaisonSociale)
	obligatoire("client.numRC", r.Client.NumRC)
	obligatoire("client.nom", r.Client.Nom)
	obligatoire("client.prenom", r.Client.Prenom)
	obligatoire("client.dateNaissance", r.Client.DateNaissance)
	obligatoire("client.typePiece", r.Client.TypePiece)
	obligatoire("client.numeroPiece", r.Client.NumeroPiece)
	if r.TypePortabilite != "PREPAID" && r.TypePortabilite != "POSTPAID" {
		champs = append(champs, entity.FieldFault{
			ObjectName: "demandeEntrepriseDTO", Field: "typePortabilite",
			Message: "doit valoir PREPAID ou POSTPAID",
		})
	}
	// numerosFlotte vide n'est pas traité ici : le guide lui réserve le code
	// FLOTTE_VIDE (§9), rendu par le handler.
	return champs
}

// --- Restitution -------------------------------------------------------------

type reqRestitution struct {
	Numero string `json:"numero"`
}

func (d *Deps) postDemandeRestitution(c *gin.Context) {
	var req reqRestitution
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, entity.InvalidJSONFormat())
		return
	}
	if !motifMSISDN.MatchString(req.Numero) {
		d.R.Fail(c, entity.Validation(entity.FieldFault{
			ObjectName: "demandeRestitutionDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}))
		return
	}

	etat, err := d.etatNumero(c, req.Numero)
	if err != nil {
		d.R.Fail(c, err)
		return
	}

	// [HYP] Le guide ne tranche pas la répartition des rôles sur une restitution ;
	// le projet a choisi que l'appelant doit être l'opérateur d'origine du numéro
	// et devient destinataire (il récupère le numéro). Voir §9.4 de la spec.
	appelant := Appelant(c)
	if etat.OriginOperatorID != appelant.OperatorID {
		d.R.Fail(c, entity.RequestAccessDenied(
			"Seul l'opérateur d'origine du numéro peut demander sa restitution."))
		return
	}

	if e := entity.CheckRestitutionEligibility(etat, entity.DelayBeforeRestitution); e != nil {
		d.R.Fail(c, e)
		return
	}

	id := oid.New()
	maintenant := time.Now()

	tx, err2 := d.DB.Pool.Begin(c)
	if err2 != nil {
		d.R.Fail(c, entity.InternalError("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	// operateur_source_id = détenteur actuel (il rend le numéro) ;
	// operateur_destinataire_id = createur_operateur_id = opérateur d'origine
	// (appelant, il récupère le numéro). routage_info et processus restent NULL :
	// une restitution n'a ni prefixe de routage ni dimension PREPAID/POSTPAID
	// avant sa COMPLETION.
	if _, err := tx.Exec(c,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, date_demande, date_debut_etape)
		 VALUES ($1,$2,'PARTICULIER','RESTITUTION','EN_COURS','ACCEPTATION','EN_COURS',
		         $3,$4,$4,$5,$5)`,
		id, req.Numero, etat.CurrentOperatorID, etat.OriginOperatorID, maintenant); err != nil {
		d.R.Fail(c, entity.InternalError("création de la demande"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_numero (demande_id, numero, statut)
		 VALUES ($1,$2,'EN_COURS')`, id, req.Numero); err != nil {
		d.R.Fail(c, entity.InternalError("enregistrement du numéro"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, entity.InternalError("validation de la transaction"))
		return
	}

	dto, err3 := d.demandeDTO(c, id)
	if err3 != nil {
		d.R.Fail(c, entity.InternalError("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusCreated, "Demande de restitution créée avec succès", dto)
}
