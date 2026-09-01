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
