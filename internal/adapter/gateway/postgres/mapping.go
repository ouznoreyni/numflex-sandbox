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
// joined in by ByUsername.
//
//	Go field       SQL column   Notes
//	-----------    ----------   -----
//	OperatorID     id
//	OperatorName   nom
