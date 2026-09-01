// Package postgres — this file is the border post between the English Go
// identifiers used from internal/entity and internal/usecase/port upward,
// and the French SQL vocabulary that stays frozen (table and column names,
// per the project's global constraint). It carries no logic: each block
// below documents, for one table, which column backs which field. Later
// tasks extend this file table by table as their gateways land.

package postgres

// Table otp — port.OneTimePassword (internal/usecase/port/gateway.go).
//
//	Go field    SQL column   Notes
//	--------    ----------   -----
//	MSISDN      numero       primary key
//	Code        code
//	ExpiresAt   expire_a
//	Attempts    tentatives   reset to 0 on Upsert's ON CONFLICT branch
//	Consumed    consomme     reset to false on Upsert's ON CONFLICT branch
//	—           cree_le      write-only: set on Upsert, never read back

// Table utilisateur — entity.Caller, via port.UserGateway (internal/usecase/port/gateway.go).
//
//	Go field    SQL column      Notes
//	--------    ----------      -----
//	UserID      id              read by ByUsername only
//	Username    username        unique; both methods filter on it
//	—           password_hash   write-only from this gateway's side: read by
//	                            ByCredentials, compared with bcrypt, never returned
//	Roles       roles           TEXT[]; populated by ByCredentials only — token
//	                            issuance is the only consumer of a caller's roles
//	—           operateur_id    join key into operateur, read by ByUsername only

// Table operateur — the OperatorID/OperatorName half of entity.Caller,
// joined in by ByUsername, and separately the full row backing
// entity.Operator (internal/usecase/port/gateway.go's ReferenceGateway).
//
//	Go field       SQL column   Notes
//	-----------    ----------   -----
//	OperatorID     id           entity.Caller's half
//	OperatorName   nom          entity.Caller's half
//	ID             id           entity.Operator's half
//	Name           nom          entity.Operator's half

// Table motif_rejet — entity.RejectionReason (internal/usecase/port/gateway.go).
// port.ReferenceGateway.RejectionReasonExists (Task 14) reads only id, an
// EXISTS check with no Go-side mapping of its own.
//
//	Go field   SQL column   Notes
//	--------   ----------   -----
//	ID         id
//	Reason     motif        ANO-009: the JSON field stays "motif", not "libelle"

// Table type_demande — entity.RequestTypeRef (internal/usecase/port/gateway.go).
//
//	Go field   SQL column   Notes
//	--------   ----------   -----
//	ID         id
//	Type       type

// Table processus — entity.Process (internal/usecase/port/gateway.go).
//
//	Go field   SQL column   Notes
//	--------   ----------   -----
//	ID         id
//	Type       type

// Table type_incident — entity.IncidentType (internal/usecase/port/gateway.go).
//
//	Go field       SQL column     Notes
//	-----------    ------------   -----
//	ID             id
//	Label          libelle
//	SystemLocked   fige_systeme

// Table numero — entity.NumberState, via port.NumberGateway.State
// (internal/usecase/port/gateway.go).
//
//	Go field            SQL column             Notes
//	--------            ----------             -----
//	CurrentOperatorID   operateur_actuel_id
//	OriginOperatorID    operateur_origine_id
//	LastPortingDate     date_dernier_portage
//	AlreadyRestituted   deja_restitue
//	RequestInProgress   —                      computed: EXISTS an EN_COURS,
//	                                            non-excluded, non-REJETE
//	                                            demande_numero row for this
//	                                            numero — not a column

// Table demande — port.CreateRequestInput / port.RequestView, via
// port.RequestGateway (internal/usecase/port/gateway.go). Statut, étape and
// leur statut sont fixés en dur à la création ('EN_COURS', 'ACCEPTATION',
// 'EN_COURS') : ni Create ni sa colonne ne les paramètrent.
//
//	Go field               SQL column                  Notes
//	--------               ----------                  -----
//	ID                     id
//	MSISDN                 numero
//	SubscriberType         type_abonne                 PARTICULIER/ENTREPRISE
//	RequestType            type_demande                PORTAGE/RESTITUTION
//	Status                 statut_demande               read-only on RequestView
//	CurrentStep            etape_actuelle                read-only on RequestView
//	CurrentStepStatus      statut_etape_actuel           read-only on RequestView
//	SourceOperatorID       operateur_source_id
//	RecipientOperatorID    operateur_destinataire_id
//	CreatorOperatorID      createur_operateur_id         write-only: always
//	                                                      equal to RecipientOperatorID
//	                                                      today, kept distinct
//	                                                      for a future capacity
//	                                                      where it might diverge
//	Processus              processus                     nil ⇒ NULL (restitution)
//	RoutingInfo            routage_info                  nil ⇒ NULL (restitution)
//	RequestDate            date_demande                  also written to
//	                                                      date_debut_etape
//	—                      date_debut_etape               write-only: RequestDate again
//	CompletionDate         date_finalisation              read-only on RequestView;
//	                                                      never written at creation
//
// Task 14 (acceptance) adds three more RequestGateway methods against this
// same table, none of them mapped field by field like the block above
// because each writes or reads a single column:
//
//	ByID          reads type_demande, type_abonne, statut_demande,
//	              etape_actuelle, statut_etape_actuel, operateur_source_id,
//	              operateur_destinataire_id, createur_operateur_id and
//	              transition_prevue_a — the same six-plus columns
//	              chargerDemande read, onto entity.PortingRequest rather
//	              than RequestView.
//	SetComment    writes commentaire alone.
//	Reject        writes statut_demande, statut_etape_actuel,
//	              date_finalisation, motif_rejet_id and commentaire, plus
//	              one etape_historique row (see below).

// Table demande_numero — port.RequestNumberInput / port.ExcludedNumberInput,
// via port.RequestGateway (internal/usecase/port/gateway.go). Task 14
// (acceptance) adds three read/write methods against the same statut and
// motif_rejet_id columns a fleet's creation-time exclusion already uses:
// NumberBelongs (EXISTS on demande_id + numero), RejectNumber (writes
// statut = 'REJETE' and motif_rejet_id, mirroring AddExcludedNumber's own
// REJETE row) and HasActiveNumber (EXISTS a row whose statut isn't 'REJETE').
//
//	Go field                     SQL column               Notes
//	--------                     ----------                -----
//	RequestID                    demande_id
//	MSISDN                       numero
//	RoutingInfo                  routage_info              nil ⇒ NULL (restitution,
//	                                                        and every excluded row)
//	—                            statut                    'EN_COURS' (retained) or
//	                                                        'REJETE' (excluded or
//	                                                        acceptance-rejected)
//	—                            exclu                     false (retained) or true —
//	                                                        acceptance's RejectNumber
//	                                                        never sets this column,
//	                                                        only creation-time
//	                                                        exclusion does
//	Reason (ExcludedNumberInput) raison_exclusion
//	ErrorCode ( "  )             code_erreur_exclusion
//	—                            motif_rejet_id            written by RejectNumber
//	                                                        (acceptance), NULL for a
//	                                                        creation-time exclusion

// Table etape_historique — written only by RequestGateway.Reject (Task 14):
// no engine transition ever produces this row for a rejection, since R-10's
// convergence only governs acceptance and later steps closing out normally.
//
//	SQL column     Notes
//	----------     -----
//	demande_id     the rejected request
//	etape          copied from demande.etape_actuelle at the moment of rejection
//	statut         fixed 'TERMINE'
//	operateur_id   the caller who rejected
//	origine        fixed 'ACTION'
//	commentaire    NULL ⇒ empty
//	date_debut     copied from demande.date_debut_etape
//	date_fin       the rejection instant

// Table demande_client — port.ClientInput / port.ClientView, via
// port.RequestGateway (internal/usecase/port/gateway.go). Absent for a
// restitution: AddClient is simply never called.
//
//	Go field      SQL column       Notes
//	--------      ----------       -----
//	LastName      nom
//	FirstName     prenom
//	BirthDate     date_naissance   ClientInput carries the yyyy-mm-dd string
//	                               bound from JSON as-is; ClientView reads it
//	                               back as *time.Time
//	BirthPlace    lieu_naissance
//	IDType        type_piece
//	IDNumber      numero_piece
//	CompanyName   raison_sociale   nil outside ENTREPRISE
//	RCNumber      num_rc           nil outside ENTREPRISE

// port.QueryGateway (internal/usecase/port/gateway.go) reads demande —
// same table and columns as port.RequestGateway above — filtered per queue
// rather than mapped field by field; the seven predicates (§7.6-§7.8) live
// in internal/adapter/gateway/postgres/query_gateway.go itself, not
// repeated here. It also reads:
//
// Table confirmation — read-only, joined or EXISTS-checked by ToConfirm and
// AlreadyConfirmed; nothing in port.QueryGateway writes it (a confirmation
// is recorded by a capability not yet migrated).
//
//	SQL column     Notes
//	----------     -----
//	demande_id     joined to demande.id
//	operateur_id   the confirming operator
