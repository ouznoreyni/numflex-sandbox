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

// Table demande_numero — port.RequestNumberInput / port.ExcludedNumberInput,
// via port.RequestGateway (internal/usecase/port/gateway.go).
//
//	Go field                     SQL column               Notes
//	--------                     ----------                -----
//	RequestID                    demande_id
//	MSISDN                       numero
//	RoutingInfo                  routage_info              nil ⇒ NULL (restitution,
//	                                                        and every excluded row)
//	—                            statut                    'EN_COURS' (retained) or
//	                                                        'REJETE' (excluded), fixed
//	—                            exclu                     false (retained) or true
//	Reason (ExcludedNumberInput) raison_exclusion
//	ErrorCode ( "  )             code_erreur_exclusion

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
