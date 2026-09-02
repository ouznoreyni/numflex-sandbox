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

// --- Individual -------------------------------------------------------------

type creationClientDTO struct {
	Name             string `json:"nom"`
	Prenom           string `json:"prenom"`
	DateNaissance    string `json:"dateNaissance"`
	LieuNaissance    string `json:"lieuNaissance"`
	TypePiece        string `json:"typePiece"`
	IDDocumentNumber string `json:"numeroPiece"`
	CompanyName      string `json:"raisonSociale"`
	NumRC            string `json:"numRC"`
}

type individualRequestDTO struct {
	MSISDN              string            `json:"numero"`
	OtpCode             string            `json:"otpCode"`
	SourceOperatorID    string            `json:"operateurSourceId"`
	RecipientOperatorID string            `json:"operateurDestinataireId"`
	TypePortabilite     string            `json:"typePortabilite"`
	Client              creationClientDTO `json:"client"`
}

// Individual handles POST /demandes/particulier.
func (ctl *CreationController) Individual(c *gin.Context) {
	var req individualRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if fields := validateIndividual(req); len(fields) > 0 {
		render(c, ctl.pres.Failure(entity.Validation(fields...), c.Request.URL.Path))
		return
	}

	view, fault := ctl.individual.Execute(c.Request.Context(), creation.CreateIndividualRequestInput{
		MSISDN: req.MSISDN, OTPCode: req.OtpCode,
		SourceOperatorID: req.SourceOperatorID, RecipientOperatorID: req.RecipientOperatorID,
		Process: req.TypePortabilite,
		Client: creation.ClientInput{
			LastName: req.Client.Name, FirstName: req.Client.Prenom,
			BirthDate: req.Client.DateNaissance, BirthPlace: req.Client.LieuNaissance,
			IDType: req.Client.TypePiece, IDNumber: req.Client.IDDocumentNumber,
		},
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	render(c, ctl.pres.Success(http.StatusCreated, "Demande particulier créée avec succès",
		requestViewDTO(ctl.clock, view)))
}

// validateIndividual reproduces the platform's validation, including its
// deviation from the guide: lieuNaissance is documented optional but
// rejected when absent (ANO-010).
func validateIndividual(r individualRequestDTO) []entity.FieldFault {
	var fields []entity.FieldFault
	required := func(field, value string) {
		if value == "" {
			fields = append(fields, entity.FieldFault{
				ObjectName: "demandeParticulierDTO", Field: field,
				Message: "ne doit pas être vide",
			})
		}
	}
	if !msisdnPattern.MatchString(r.MSISDN) {
		fields = append(fields, entity.FieldFault{
			ObjectName: "demandeParticulierDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		})
	}
	required("otpCode", r.OtpCode)
	required("operateurSourceId", r.SourceOperatorID)
	required("operateurDestinataireId", r.RecipientOperatorID)
	required("client.nom", r.Client.Name)
	required("client.prenom", r.Client.Prenom)
	required("client.dateNaissance", r.Client.DateNaissance)
	required("client.lieuNaissance", r.Client.LieuNaissance)
	required("client.typePiece", r.Client.TypePiece)
	required("client.numeroPiece", r.Client.IDDocumentNumber)
	if r.TypePortabilite != "PREPAID" && r.TypePortabilite != "POSTPAID" {
		fields = append(fields, entity.FieldFault{
			ObjectName: "demandeParticulierDTO", Field: "typePortabilite",
			Message: "doit valoir PREPAID ou POSTPAID",
		})
	}
	return fields
}

// --- Enterprise (fleet) ------------------------------------------------------

type enterpriseRequestDTO struct {
	FleetHolderMSISDN   string            `json:"numeroPorteurFlotte"`
	OtpCode             string            `json:"otpCode"`
	SourceOperatorID    string            `json:"operateurSourceId"`
	RecipientOperatorID string            `json:"operateurDestinataireId"`
	TypePortabilite     string            `json:"typePortabilite"`
	FleetMSISDNs        []string          `json:"numerosFlotte"`
	Client              creationClientDTO `json:"client"`
}

// excludedNumberDTO carries the reason a fleet number was excluded (§7.4).
type excludedNumberDTO struct {
	MSISDN    string `json:"numero"`
	Reason    string `json:"raison"`
	ErrorCode string `json:"codeErreur"`
}

// Enterprise handles POST /demandes/entreprise.
func (ctl *CreationController) Enterprise(c *gin.Context) {
	var req enterpriseRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if fields := validateEnterprise(req); len(fields) > 0 {
		render(c, ctl.pres.Failure(entity.Validation(fields...), c.Request.URL.Path))
		return
	}

	out, fault := ctl.enterprise.Execute(c.Request.Context(), creation.CreateEnterpriseRequestInput{
		FleetMSISDN: req.FleetHolderMSISDN, OTPCode: req.OtpCode,
		SourceOperatorID: req.SourceOperatorID, RecipientOperatorID: req.RecipientOperatorID,
		Process: req.TypePortabilite, FleetNumbers: req.FleetMSISDNs,
		Client: creation.ClientInput{
			LastName: req.Client.Name, FirstName: req.Client.Prenom,
			BirthDate: req.Client.DateNaissance, BirthPlace: req.Client.LieuNaissance,
			IDType: req.Client.TypePiece, IDNumber: req.Client.IDDocumentNumber,
			CompanyName: req.Client.CompanyName, RCNumber: req.Client.NumRC,
		},
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	excluded := make([]excludedNumberDTO, 0, len(out.Excluded))
	for _, ex := range out.Excluded {
		excluded = append(excluded, excludedNumberDTO{MSISDN: ex.MSISDN, Reason: ex.Reason, ErrorCode: ex.ErrorCode})
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
		"numerosExclus":      excluded,
	}
	if len(excluded) > 0 {
		data["avertissement"] = fmt.Sprintf("%d numéro(s) exclu(s) de la demande.", len(excluded))
	}
	render(c, ctl.pres.Success(http.StatusCreated, "Demande flotte créée", data))
}

// validateEnterprise reproduces the shape validation of a fleet request: the
// same required fields as an individual request, but numeroPorteurFlotte in
// place of numero, raisonSociale/numRC in place of a bare identity, and
// numerosFlotte whose emptiness is the FLOTTE_VIDE code, outside this
// function (it belongs to the interactor).
func validateEnterprise(r enterpriseRequestDTO) []entity.FieldFault {
	var fields []entity.FieldFault
	required := func(field, value string) {
		if value == "" {
			fields = append(fields, entity.FieldFault{
				ObjectName: "demandeEntrepriseDTO", Field: field,
				Message: "ne doit pas être vide",
			})
		}
	}
	if !msisdnPattern.MatchString(r.FleetHolderMSISDN) {
		fields = append(fields, entity.FieldFault{
			ObjectName: "demandeEntrepriseDTO", Field: "numeroPorteurFlotte",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		})
	}
	required("otpCode", r.OtpCode)
	required("operateurSourceId", r.SourceOperatorID)
	required("operateurDestinataireId", r.RecipientOperatorID)
	required("client.raisonSociale", r.Client.CompanyName)
	required("client.numRC", r.Client.NumRC)
	required("client.nom", r.Client.Name)
	required("client.prenom", r.Client.Prenom)
	required("client.dateNaissance", r.Client.DateNaissance)
	required("client.typePiece", r.Client.TypePiece)
	required("client.numeroPiece", r.Client.IDDocumentNumber)
	if r.TypePortabilite != "PREPAID" && r.TypePortabilite != "POSTPAID" {
		fields = append(fields, entity.FieldFault{
			ObjectName: "demandeEntrepriseDTO", Field: "typePortabilite",
			Message: "doit valoir PREPAID ou POSTPAID",
		})
	}
	return fields
}

// --- Restitution ---------------------------------------------------------------

type restitutionRequestDTO struct {
	MSISDN string `json:"numero"`
}

// Restitution handles POST /demandes/restitution.
func (ctl *CreationController) Restitution(c *gin.Context) {
	var req restitutionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if !msisdnPattern.MatchString(req.MSISDN) {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "demandeRestitutionDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}), c.Request.URL.Path))
		return
	}

	view, fault := ctl.restitution.Execute(c.Request.Context(),
		creation.CreateRestitutionRequestInput{MSISDN: req.MSISDN})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	render(c, ctl.pres.Success(http.StatusCreated, "Demande de restitution créée avec succès",
		requestViewDTO(ctl.clock, view)))
}
