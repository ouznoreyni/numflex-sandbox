package entity

// --- OTP (§9) ---------------------------------------------------------------
// ANO-014 : en recette ces situations sortent en texte libre. Les messages
// ci-dessous sont ceux qui ont été mesurés.

func OTPInvalid() *Fault {
	return New(FaultState, "OTP_INVALID", "Code OTP incorrect")
}

func OTPExpired() *Fault {
	f := New(FaultState, "OTP_EXPIRED", "Code expiré (délai de 5 minutes dépassé)")
	f.RealDetail = "Le code OTP a expiré"
	return f
}

func OTPAlreadyUsed() *Fault {
	return New(FaultState, "OTP_ALREADY_USED", "Code déjà utilisé")
}

func OTPMaxAttempts() *Fault {
	return New(FaultState, "OTP_MAX_ATTEMPTS", "Nombre maximum de tentatives atteint (3 essais)")
}

func OTPAbsent() *Fault {
	f := New(FaultState, "OTP_INVALID", "Aucun OTP actif pour ce numéro")
	f.RealDetail = "Aucun OTP actif pour ce numéro"
	return f
}

// --- Portage (§9) -----------------------------------------------------------

func NumberAlreadyAtRecipient() *Fault {
	return New(FaultState, "NUMERO_DEJA_CHEZ_DESTINATAIRE",
		"Le numéro est déjà chez l'opérateur destinataire")
}

func IncorrectSourceOperator() *Fault {
	return New(FaultState, "OPERATEUR_SOURCE_INCORRECT",
		"Le numéro n'appartient pas à l'opérateur source indiqué")
}

func RequestAlreadyInProgressForNumber() *Fault {
	return New(FaultState, "DEMANDE_EN_COURS_POUR_NUMERO",
		"Demande déjà en cours pour ce numéro")
}

// PortingDelayNotRespected — ANO-002 : la couche de validation est franchie et
// l'exception survient dans la logique métier. Le client reçoit une panne serveur
// là où le catalogue prévoit un refus métier.
func PortingDelayNotRespected() *Fault {
	f := New(FaultState, "DELAI_PORTAGE_NON_RESPECTE",
		"Le délai minimum entre deux portages n'est pas respecté")
	f.RealDetail = "Unexpected runtime exception"
	return f
}

// --- Restitution / reverse (§9) ---------------------------------------------

func NumberNotPorted() *Fault {
	return New(FaultState, "NUMERO_NON_PORTE",
		"Le numéro n'a pas été porté, pas de restitution/reverse possible")
}

func NumberAlreadyRestituted() *Fault {
	return New(FaultState, "NUMERO_DEJA_RESTITUE", "Ce numéro a déjà été restitué")
}

// RestitutionDelayNotRespected — ANO-020 : une erreur 400 sérialisée en chaîne,
// encapsulée dans une 500. Le code exploitable existe mais reste enterré.
func RestitutionDelayNotRespected() *Fault {
	f := New(FaultState, "DELAI_RESTITUTION_NON_RESPECTE",
		"Le délai de 6 mois minimum n'est pas écoulé")
	f.RealDetail = `400 BAD_REQUEST "{"type":"https://www.jhipster.tech/problem/problem-with-message",` +
		`"title":"Bad Request","status":400,"detail":"error.numeroRestitutionTooEarly"}"`
	return f
}

// --- Workflow (§9) ----------------------------------------------------------

func InvalidStep(message string) *Fault {
	return New(FaultState, "ETAPE_INVALIDE", message)
}

func RejectionReasonRequired() *Fault {
	return New(FaultState, "MOTIF_REJET_OBLIGATOIRE",
		"Un motif de rejet est obligatoire pour rejeter une demande")
}

func RequestNotFound() *Fault {
	return New(FaultNotFound, "DEMANDE_NON_TROUVEE", "Demande introuvable")
}

func RequestAccessDenied(message string) *Fault {
	return New(FaultAccess, "DEMANDE_ACCES_REFUSE", message)
}

// --- Flotte (§9) ------------------------------------------------------------

func FleetEmpty() *Fault {
	return New(FaultValidation, "FLOTTE_VIDE", "La liste des numéros de flotte est vide")
}

func FleetMixedOperators() *Fault {
	return New(FaultState, "FLOTTE_OPERATEURS_MIXTES",
		"Les numéros appartiennent à des opérateurs différents")
}

func NoEligibleNumber() *Fault {
	return New(FaultState, "AUCUN_NUMERO_ELIGIBLE",
		"Aucun numéro de la flotte n'est éligible au portage")
}

// --- Accès et validation (§9) -----------------------------------------------

func AccessForbidden() *Fault {
	return New(FaultAccess, "ACCES_INTERDIT",
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.")
}

func OperatorNotFound() *Fault {
	return New(FaultAccess, "OPERATEUR_NON_TROUVE",
		"Votre compte n'est pas associé à un opérateur")
}

func ValidationFailed(message string) *Fault {
	return New(FaultValidation, "VALIDATION_ECHOUEE", message)
}

func InvalidJSONFormat() *Fault {
	return New(FaultValidation, "FORMAT_JSON_INVALIDE",
		"Le corps de la requête n'est pas un JSON valide")
}

func InternalError(message string) *Fault {
	return New(FaultInternal, "ERREUR_INTERNE", message)
}

// --- Authentification ---------------------------------------------------

// BadCredentials is AuthenticateInteractor's answer to an unknown username or
// a wrong password. ANO-016 : the real platform renders this outside the
// ARTP envelope entirely, in JHipster's own "Bad credentials" problem+json —
// this Fault's Code and Message never reach a client, only its Kind, which
// the controller uses to pick that fixed rendering over the presenter.
func BadCredentials() *Fault {
	return New(FaultAccess, "IDENTIFIANTS_INVALIDES", "Nom d'utilisateur ou mot de passe incorrect")
}
