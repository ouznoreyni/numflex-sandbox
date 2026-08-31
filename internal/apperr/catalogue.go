package apperr

// --- OTP (§9) ---------------------------------------------------------------
// ANO-014 : en recette ces situations sortent en texte libre. Les messages
// ci-dessous sont ceux qui ont été mesurés.

func OTPInvalid() *Error {
	return New(KindEtat, "OTP_INVALID", "Code OTP incorrect")
}

func OTPExpired() *Error {
	e := New(KindEtat, "OTP_EXPIRED", "Code expiré (délai de 5 minutes dépassé)")
	e.RealDetail = "Le code OTP a expiré"
	return e
}

func OTPAlreadyUsed() *Error {
	return New(KindEtat, "OTP_ALREADY_USED", "Code déjà utilisé")
}

func OTPMaxAttempts() *Error {
	return New(KindEtat, "OTP_MAX_ATTEMPTS", "Nombre maximum de tentatives atteint (3 essais)")
}

func OTPAbsent() *Error {
	e := New(KindEtat, "OTP_INVALID", "Aucun OTP actif pour ce numéro")
	e.RealDetail = "Aucun OTP actif pour ce numéro"
	return e
}

// --- Portage (§9) -----------------------------------------------------------

func NumeroDejaChezDestinataire() *Error {
	return New(KindEtat, "NUMERO_DEJA_CHEZ_DESTINATAIRE",
		"Le numéro est déjà chez l'opérateur destinataire")
}

func OperateurSourceIncorrect() *Error {
	return New(KindEtat, "OPERATEUR_SOURCE_INCORRECT",
		"Le numéro n'appartient pas à l'opérateur source indiqué")
}

func DemandeEnCoursPourNumero() *Error {
	return New(KindEtat, "DEMANDE_EN_COURS_POUR_NUMERO",
		"Une demande est déjà en cours pour ce numéro")
}

// DelaiPortageNonRespecte — ANO-002 : la couche de validation est franchie et
// l'exception survient dans la logique métier. Le client reçoit une panne serveur
// là où le catalogue prévoit un refus métier.
func DelaiPortageNonRespecte() *Error {
	e := New(KindEtat, "DELAI_PORTAGE_NON_RESPECTE",
		"Le délai minimum entre deux portages n'est pas respecté")
	e.RealDetail = "Unexpected runtime exception"
	return e
}

// --- Restitution / reverse (§9) ---------------------------------------------

func NumeroNonPorte() *Error {
	return New(KindEtat, "NUMERO_NON_PORTE",
		"Le numéro n'a pas été porté, pas de restitution/reverse possible")
}

func NumeroDejaRestitue() *Error {
	return New(KindEtat, "NUMERO_DEJA_RESTITUE", "Ce numéro a déjà été restitué")
}

// DelaiRestitutionNonRespecte — ANO-020 : une erreur 400 sérialisée en chaîne,
// encapsulée dans une 500. Le code exploitable existe mais reste enterré.
func DelaiRestitutionNonRespecte() *Error {
	e := New(KindEtat, "DELAI_RESTITUTION_NON_RESPECTE",
		"Le délai de 6 mois minimum n'est pas écoulé")
	e.RealDetail = `400 BAD_REQUEST "{"type":"https://www.jhipster.tech/problem/problem-with-message",` +
		`"title":"Bad Request","status":400,"detail":"error.numeroRestitutionTooEarly"}"`
	return e
}

// --- Workflow (§9) ----------------------------------------------------------

func EtapeInvalide(message string) *Error {
	return New(KindEtat, "ETAPE_INVALIDE", message)
}

func MotifRejetObligatoire() *Error {
	return New(KindEtat, "MOTIF_REJET_OBLIGATOIRE",
		"Un motif de rejet est obligatoire pour rejeter une demande")
}

func DemandeNonTrouvee() *Error {
	return New(KindIntrouvable, "DEMANDE_NON_TROUVEE", "Demande introuvable")
}

func DemandeAccesRefuse(message string) *Error {
	return New(KindAcces, "DEMANDE_ACCES_REFUSE", message)
}

// --- Flotte (§9) ------------------------------------------------------------

func FlotteVide() *Error {
	return New(KindValidation, "FLOTTE_VIDE", "La liste des numéros de flotte est vide")
}

func FlotteOperateursMixtes() *Error {
	return New(KindEtat, "FLOTTE_OPERATEURS_MIXTES",
		"Les numéros appartiennent à des opérateurs différents")
}

func AucunNumeroEligible() *Error {
	return New(KindEtat, "AUCUN_NUMERO_ELIGIBLE",
		"Aucun numéro de la flotte n'est éligible au portage")
}

// --- Accès et validation (§9) -----------------------------------------------

func AccesInterdit() *Error {
	return New(KindAcces, "ACCES_INTERDIT",
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.")
}

func OperateurNonTrouve() *Error {
	return New(KindAcces, "OPERATEUR_NON_TROUVE",
		"Votre compte n'est pas associé à un opérateur")
}

func ValidationEchouee(message string) *Error {
	return New(KindValidation, "VALIDATION_ECHOUEE", message)
}

func FormatJSONInvalide() *Error {
	return New(KindValidation, "FORMAT_JSON_INVALIDE",
		"Le corps de la requête n'est pas un JSON valide")
}

func ErreurInterne(message string) *Error {
	return New(KindInterne, "ERREUR_INTERNE", message)
}
