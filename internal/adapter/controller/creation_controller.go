package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/creation"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// CreationController is the interface-adapter for the three request-creation
// routes: /demandes/particulier, /demandes/entreprise, /demandes/restitution.
// Each handler binds JSON, applies the field-shape validation that is not an
// interactor's business (a required field, a regular expression), delegates
// to a boundary, and renders the result — the same division OTPController
// and ReferenceController already establish.
type CreationController struct {
	individual  creation.CreateIndividualRequestBoundary
	enterprise  creation.CreateEnterpriseRequestBoundary
	restitution creation.CreateRestitutionRequestBoundary
	pres        presenter.Presenter
	// clock formats the two timestamps a particulier/restitution response
	// carries (dateDemande, dateFinalisation) — the same skew presenter.Real
	// and presenter.Contract apply to every other rendered timestamp, needed
	// here too because building this response map is the controller's job,
	// not the presenter's.
	clock port.Clock
}

// NewCreationController wires a controller against the three boundaries, a
// presenter and a clock.
func NewCreationController(
	individual creation.CreateIndividualRequestBoundary,
	enterprise creation.CreateEnterpriseRequestBoundary,
	restitution creation.CreateRestitutionRequestBoundary,
	p presenter.Presenter,
	clock port.Clock,
) *CreationController {
	return &CreationController{
		individual: individual, enterprise: enterprise, restitution: restitution,
		pres: p, clock: clock,
	}
}

// --- Particulier -------------------------------------------------------------

type creationClientDTO struct {
	Nom           string `json:"nom"`
	Prenom        string `json:"prenom"`
	DateNaissance string `json:"dateNaissance"`
	LieuNaissance string `json:"lieuNaissance"`
	TypePiece     string `json:"typePiece"`
	NumeroPiece   string `json:"numeroPiece"`
	RaisonSociale string `json:"raisonSociale"`
	NumRC         string `json:"numRC"`
}

type demandeParticulierRequest struct {
	Numero                  string            `json:"numero"`
	OtpCode                 string            `json:"otpCode"`
	OperateurSourceID       string            `json:"operateurSourceId"`
	OperateurDestinataireID string            `json:"operateurDestinataireId"`
	TypePortabilite         string            `json:"typePortabilite"`
	Client                  creationClientDTO `json:"client"`
}

// Particulier handles POST /demandes/particulier.
func (ctl *CreationController) Particulier(c *gin.Context) {
	var req demandeParticulierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if champs := validerParticulier(req); len(champs) > 0 {
		render(c, ctl.pres.Failure(entity.Validation(champs...), c.Request.URL.Path))
		return
	}

	view, fault := ctl.individual.Execute(c.Request.Context(), creation.CreateIndividualRequestInput{
		MSISDN: req.Numero, OTPCode: req.OtpCode,
		SourceOperatorID: req.OperateurSourceID, RecipientOperatorID: req.OperateurDestinataireID,
		Processus: req.TypePortabilite,
		Client: creation.ClientInput{
			LastName: req.Client.Nom, FirstName: req.Client.Prenom,
			BirthDate: req.Client.DateNaissance, BirthPlace: req.Client.LieuNaissance,
			IDType: req.Client.TypePiece, IDNumber: req.Client.NumeroPiece,
		},
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	render(c, ctl.pres.Success(http.StatusCreated, "Demande particulier créée avec succès",
		requestViewDTO(ctl.clock, view)))
}

// validerParticulier reproduit la validation de la plateforme, y compris son
// écart au guide : lieuNaissance est documenté facultatif mais rejeté si
// absent (ANO-010).
func validerParticulier(r demandeParticulierRequest) []entity.FieldFault {
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

// --- Entreprise (flotte) ------------------------------------------------------

type demandeEntrepriseRequest struct {
	NumeroPorteurFlotte     string            `json:"numeroPorteurFlotte"`
	OtpCode                 string            `json:"otpCode"`
	OperateurSourceID       string            `json:"operateurSourceId"`
	OperateurDestinataireID string            `json:"operateurDestinataireId"`
	TypePortabilite         string            `json:"typePortabilite"`
	NumerosFlotte           []string          `json:"numerosFlotte"`
	Client                  creationClientDTO `json:"client"`
}

// numeroExcluDTO porte le motif d'exclusion d'un numéro de la flotte (§7.4).
type numeroExcluDTO struct {
	Numero     string `json:"numero"`
	Raison     string `json:"raison"`
	CodeErreur string `json:"codeErreur"`
}

// Entreprise handles POST /demandes/entreprise.
func (ctl *CreationController) Entreprise(c *gin.Context) {
	var req demandeEntrepriseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if champs := validerEntreprise(req); len(champs) > 0 {
		render(c, ctl.pres.Failure(entity.Validation(champs...), c.Request.URL.Path))
		return
	}

	out, fault := ctl.enterprise.Execute(c.Request.Context(), creation.CreateEnterpriseRequestInput{
		FleetMSISDN: req.NumeroPorteurFlotte, OTPCode: req.OtpCode,
		SourceOperatorID: req.OperateurSourceID, RecipientOperatorID: req.OperateurDestinataireID,
		Processus: req.TypePortabilite, FleetNumbers: req.NumerosFlotte,
		Client: creation.ClientInput{
			LastName: req.Client.Nom, FirstName: req.Client.Prenom,
			BirthDate: req.Client.DateNaissance, BirthPlace: req.Client.LieuNaissance,
			IDType: req.Client.TypePiece, IDNumber: req.Client.NumeroPiece,
			CompanyName: req.Client.RaisonSociale, RCNumber: req.Client.NumRC,
		},
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	exclus := make([]numeroExcluDTO, 0, len(out.Excluded))
	for _, ex := range out.Excluded {
		exclus = append(exclus, numeroExcluDTO{Numero: ex.MSISDN, Raison: ex.Reason, CodeErreur: ex.ErrorCode})
	}
	data := gin.H{
		"demande": gin.H{
			"id":            out.ID,
			"typeDemande":   "PORTAGE",
			"typeAbonne":    "ENTREPRISE",
			"statutDemande": "EN_COURS",
			"etapeActuelle": "ACCEPTATION",
		},
		"numerosPortesCount": out.RetainedCount,
		"numerosExclusCount": len(out.Excluded),
		"numerosExclus":      exclus,
	}
	if len(exclus) > 0 {
		data["avertissement"] = fmt.Sprintf("%d numéro(s) exclu(s) de la demande.", len(exclus))
	}
	render(c, ctl.pres.Success(http.StatusCreated, "Demande flotte créée", data))
}

// validerEntreprise reproduit la validation de forme d'une demande flotte :
// les mêmes champs obligatoires qu'une demande particulier, mais
// numeroPorteurFlotte à la place de numero, raisonSociale/numRC à la place
// d'une identité seule, et numerosFlotte dont la vacuité relève du code
// FLOTTE_VIDE, hors de cette fonction (elle appartient à l'interactor).
func validerEntreprise(r demandeEntrepriseRequest) []entity.FieldFault {
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
	return champs
}

// --- Restitution ---------------------------------------------------------------

type demandeRestitutionRequest struct {
	Numero string `json:"numero"`
}

// Restitution handles POST /demandes/restitution.
func (ctl *CreationController) Restitution(c *gin.Context) {
	var req demandeRestitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if !motifMSISDN.MatchString(req.Numero) {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "demandeRestitutionDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}), c.Request.URL.Path))
		return
	}

	view, fault := ctl.restitution.Execute(c.Request.Context(),
		creation.CreateRestitutionRequestInput{MSISDN: req.Numero})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	render(c, ctl.pres.Success(http.StatusCreated, "Demande de restitution créée avec succès",
		requestViewDTO(ctl.clock, view)))
}
