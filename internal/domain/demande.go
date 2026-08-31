// Package domain porte les règles de portabilité, sans I/O ni HTTP. Il ne connaît
// pas le mode de fidélité : une règle métier est la même dans les deux modes, seul
// son rendu change.
package domain

type Etape string

const (
	EtapeAcceptation   Etape = "ACCEPTATION"
	EtapeDesactivation Etape = "DESACTIVATION"
	EtapeActivation    Etape = "ACTIVATION"
	EtapeConfirmation  Etape = "CONFIRMATION"
	EtapeCompletion    Etape = "COMPLETION"
)

type StatutDemande string

const (
	StatutEnCours StatutDemande = "EN_COURS"
	StatutTermine StatutDemande = "TERMINE"
	StatutAnnule  StatutDemande = "ANNULE"
	// StatutRejete — [HYP] ni documenté au guide, ni observé en recette.
	StatutRejete StatutDemande = "REJETE"
)

type StatutEtape string

const (
	EtapeEnCours  StatutEtape = "EN_COURS"
	EtapeTerminee StatutEtape = "TERMINE"
	EtapeExpiree  StatutEtape = "EXPIRE"
	EtapeValidee  StatutEtape = "VALIDE"
)

type TypeDemande string

const (
	TypePortage     TypeDemande = "PORTAGE"
	TypeRestitution TypeDemande = "RESTITUTION"
	TypeReverse     TypeDemande = "REVERSE"
)

type TypeAbonne string

const (
	AbonneParticulier TypeAbonne = "PARTICULIER"
	AbonneEntreprise  TypeAbonne = "ENTREPRISE"
)

type Role int

const (
	RoleSource Role = iota
	RoleDestinataire
	RoleTous
	RoleARTP
)

type Demande struct {
	ID                      string
	Numero                  string
	TypeDemande             TypeDemande
	TypeAbonne              TypeAbonne
	StatutDemande           StatutDemande
	EtapeActuelle           Etape
	StatutEtapeActuel       StatutEtape
	OperateurSourceID       string
	OperateurDestinataireID string
	CreateurOperateurID     string
	// TransitionEnAttente vaut true entre le traitement d'une étape et sa
	// convergence effective (R-10).
	TransitionEnAttente bool
}
