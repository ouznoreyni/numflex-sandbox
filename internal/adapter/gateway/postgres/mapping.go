// mapping.go is the border post between the English Go identifiers used
// from internal/entity and internal/usecase/port upward, and the French SQL
// vocabulary that stays frozen (table and column names, per the project's
// global constraint). It carries no logic: each block below documents, for
// one table, which column backs which field.

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
//	OperatorName   name          entity.Caller's half
//	ID             id           entity.Operator's half
//	Name           name          entity.Operator's half

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
//	Label          label
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
// port.RequestGateway (internal/usecase/port/gateway.go). The status, the
// step and the step's own status are hardcoded at creation ('EN_COURS',
// 'ACCEPTATION', 'EN_COURS'): neither Create nor its column parameterizes
// them.
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
//	Process              processus                     nil ⇒ NULL (restitution)
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
//	LastName      name
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
// Table confirmation — joined or EXISTS-checked, read-only, by
// QueryGateway's ToConfirm and AlreadyConfirmed. Task 15
// (port.ConfirmationGateway, in confirmation_gateway.go) is the table's only
// writer, one row per (demande, operateur) pair; QueryGateway still never
// writes it.
//
//	Go field (ConfirmationGateway.Confirm)   SQL column     Notes
//	---------------------------------------  ----------     -----
//	requestID                                demande_id     joined to demande.id;
//	                                                        primary key with operateur_id —
//	                                                        the anti-replay guarantee
//	                                                        Confirm's 23505 branch reads
//	operatorID                               operateur_id   the confirming operator
//	comment                                  commentaire    NULLIF('') ⇒ NULL
//	now                                      date_conf
//
// Task 15's other write, port.RequestGateway.Cancel, touches only columns
// this file already maps for the table demande and etape_historique blocks
// above (Reject's own), plus clearing transition_prevue_a — no new column.

// Table reverse_request — port.ReverseCreateInput / port.ReverseView, via
// port.ReverseGateway (Task 16, reverse_gateway.go). Statut is fixed
// 'EN_ATTENTE' at creation — Create does not parameterize it.
//
//	Go field       SQL column     Notes
//	-----------    -----------    -----
//	ID             id
//	MSISDN         numero
//	OperatorID     operateur_id
//	RequestDate    date_demande
//	Status         statut         read-only on ReverseView
//	OperatorName   —              joined from operateur.name, ReverseView only
//
// Own filters on operateur_id = $1, ordered by date_demande, paginated —
// the one queue among this task's three capabilities to accept page/size,
// like the two incident lists below.

// Table incident — port.IncidentCreateInput / port.IncidentView, via
// port.IncidentGateway (Task 16, incident_gateway.go). Statut is fixed
// 'EN_COURS' at creation, then 'RESOLU' on Resolve; the anti-race guarantee
// for "one open internal incident per operator" (§7.12) comes from the
// migration's own partial unique index (incident_interne_unique_ouvert,
// on operateur_id where statut = 'EN_COURS' and fige_systeme), reported by
// Postgres as code 23505 and translated by this gateway into
// port.ErrIncidentAlreadyOpen — the same division ConfirmationGateway.Confirm
// already draws for its own anti-replay guarantee.
//
//	Go field                  SQL column               Notes
//	-----------------------   ---------------------     -----
//	ID                        id
//	OperatorID                operateur_id
//	TypeID                    type_incident_id
//	SystemLocked              fige_systeme
//	Description               description
//	OpenedAt                  date_ouverture
//	Status                    statut                    read-only on IncidentView
//	—                         date_resolution           write-only: set by Resolve
//	—                         commentaire_resolution    write-only: set by Resolve
//	TypeLabel                 —                         joined from
//	                                                     type_incident.label,
//	                                                     IncidentView only
//	OperatorName              —                         joined from operateur.name,
//	                                                     IncidentView only
//
// TypeIDFor reads type_incident.id filtered on fige_systeme — the endpoint's
// own segment decides the category, never the request body. Own filters on
// operateur_id = $1 AND fige_systeme = $2, ordered by date_ouverture,
// paginated.

// port.SandboxGateway (Task 16, sandbox_gateway.go) touches five tables this
// file already maps field by field above (demande, reverse_request, otp,
// numero) or by cascade (demande_numero, demande_client, etape_historique,
// confirmation — all four carry ON DELETE CASCADE from demande, so
// DeleteRequests alone accounts for them). No column here is new:
//
//	RequestIDsToPurge   demande.id WHERE createur_operateur_id = $1 — never
//	                    the operateur_source_id/operateur_destinataire_id
//	                    pair /mes-demandes filters on: a request belongs to
//	                    two operators at once, only its creator made it.
//	NumbersToRestore    demande.numero UNION demande_numero.numero, for the
//	                    given ids — exclus compris.
//	DeleteReverseRequests  reverse_request WHERE operateur_id = $1 OR
//	                       demande_id = ANY($2) — ahead of DeleteRequests,
//	                       since this foreign key carries no cascade.
//	DeleteOTP           otp WHERE numero = ANY($1).
//	DeleteRequests      demande WHERE id = ANY($1).
//	RestoreNumbers      numero.operateur_actuel_id set back to
//	                    operateur_origine_id, date_dernier_portage and
//	                    deja_restitue cleared, for the given msisdns.

// Task 17 (internal/usecase/platform, internal/framework/engine) adds no new
// table: every method below reads or writes columns this file already maps
// above, split out of the deleted internal/engine/{engine,transitions,reverse}.go
// so the same writes now run through port.RequestGateway / port.ReverseGateway
// / port.IncidentGateway instead of a raw *pgx.Tx.
//
//	RequestGateway.LockForTransition        demande, FOR UPDATE — the six
//	                                         columns port.RequestGateway.ByID
//	                                         already maps above, minus
//	                                         transition_prevue_a.
//	RequestGateway.CloseCurrentStep         etape_historique — same shape as
//	                                         Reject's own row (§161 block
//	                                         above), origine parameterized
//	                                         instead of fixed 'ACTION'.
//	RequestGateway.CompleteRequest          demande.{statut_demande,
//	                                         statut_etape_actuel,
//	                                         date_finalisation,
//	                                         transition_prevue_a}.
//	RequestGateway.AdvanceStep              demande.{etape_actuelle,
//	                                         statut_etape_actuel,
//	                                         date_debut_etape,
//	                                         transition_prevue_a}.
//	RequestGateway.TransferToRegistry       numero.{operateur_actuel_id,
//	                                         date_dernier_portage}, filtered
//	                                         through demande_numero
//	                                         (NOT exclu AND statut <> 'REJETE').
//	RequestGateway.ApplyRouting             demande_numero.routage_info and
//	                                         demande.routage_info.
//	RequestGateway.ApplyEndOfRequestRestitution  numero.{operateur_actuel_id,
//	                                              date_dernier_portage,
//	                                              deja_restitue} plus
//	                                              demande.routage_info.
//	RequestGateway.ScheduleTransitionAt     demande.transition_prevue_a —
//	                                         now() + make_interval(secs=>$2),
//	                                         computed database-side
//	                                         (commit 94af3f2).
//	RequestGateway.DueConvergences          demande.id WHERE statut_demande
//	                                         = 'EN_COURS' AND
//	                                         transition_prevue_a <= now().
//	RequestGateway.OverdueSteps             demande.id WHERE statut_demande
//	                                         = 'EN_COURS' AND
//	                                         transition_prevue_a IS NULL AND
//	                                         date_debut_etape +
//	                                         make_interval(...) <= asOf.
//	RequestGateway.CreateAtConfirmation     demande — same columns as Create
//	                                         above, etape_actuelle fixed
//	                                         'CONFIRMATION' instead of
//	                                         'ACCEPTATION', processus and
//	                                         routage_info left NULL.
//	RequestGateway.PendingReverseCompletions  demande joined against
//	                                           confirmation and operateur —
//	                                           see reverse_request block
//	                                           below.
//	ReverseGateway.LockPending, MarkValidated, Reject, OverdueForAutoValidation
//	                                         reverse_request — same columns
//	                                         as the block above; MarkValidated
//	                                         also writes demande_id.
//	ReverseGateway.CurrentOperatorFor       numero.operateur_actuel_id —
//	                                         read only, no write.
//	IncidentGateway.MarketFrozen            incident.{statut,fige_systeme} —
//	                                         same columns as the block above,
//	                                         read only, unfiltered on
//	                                         operateur_id: BR-012 freezes the
//	                                         whole market, not one operator.
