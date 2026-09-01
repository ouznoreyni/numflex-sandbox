package inmemory

import (
	"context"
	"sync"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// ReferenceGateway is a map-backed port.ReferenceGateway, for use case unit
// tests that must not touch a database. Task 14 (acceptance) is its first
// caller — RejectionReasonExists is the only method an acceptance interactor
// drives; the other five are stubbed so the double satisfies the interface,
// with a matching Seed* helper each in case a later capability's test needs
// one.
type ReferenceGateway struct {
	mu               sync.Mutex
	operators        []entity.Operator
	rejectionReasons []entity.RejectionReason
	requestTypes     []entity.RequestTypeRef
	processes        []entity.Process
	incidentTypes    []entity.IncidentType
}

// NewReferenceGateway returns an empty double, ready to use.
func NewReferenceGateway() *ReferenceGateway {
	return &ReferenceGateway{}
}

// SeedRejectionReason registers one motif_rejet row, read back by
// RejectionReasons and RejectionReasonExists.
func (g *ReferenceGateway) SeedRejectionReason(id, reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rejectionReasons = append(g.rejectionReasons, entity.RejectionReason{ID: id, Reason: reason})
}

func (g *ReferenceGateway) Operators(_ context.Context) ([]entity.Operator, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]entity.Operator(nil), g.operators...), nil
}

func (g *ReferenceGateway) RejectionReasons(_ context.Context) ([]entity.RejectionReason, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]entity.RejectionReason(nil), g.rejectionReasons...), nil
}

func (g *ReferenceGateway) RequestTypes(_ context.Context) ([]entity.RequestTypeRef, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]entity.RequestTypeRef(nil), g.requestTypes...), nil
}

func (g *ReferenceGateway) Processes(_ context.Context) ([]entity.Process, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]entity.Process(nil), g.processes...), nil
}

func (g *ReferenceGateway) IncidentTypes(_ context.Context) ([]entity.IncidentType, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]entity.IncidentType(nil), g.incidentTypes...), nil
}

// RejectionReasonExists answers whether id names a seeded motif_rejet row.
func (g *ReferenceGateway) RejectionReasonExists(_ context.Context, id string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.rejectionReasons {
		if r.ID == id {
			return true, nil
		}
	}
	return false, nil
}
