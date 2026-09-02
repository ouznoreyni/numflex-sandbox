// Package otp holds the two use cases behind POST /otp/send and
// POST /otp/verify: issuing a challenge and verifying it (5 minutes,
// 3 attempts). It is the capability that set the pattern every other one
// in this layer follows.
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: pgx, Gin, or how a code reaches the subscriber. The
// code's storage is port.OTPGateway and its instant is port.Clock.
package otp
