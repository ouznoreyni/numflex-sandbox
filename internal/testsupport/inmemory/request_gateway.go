package inmemory

import (
	"context"
	"errors"
	"sync"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// RequestGateway is a map-backed port.RequestGateway, for use case unit
// tests that must not touch a database. FailCreate, when set, makes Create
// return it instead of writing — the seam a use-case-level test uses to
// exercise the "insert fails" branch without a real transaction; the
// transaction *guarantee* itself (that a failed Create really leaves the
// OTP unconsumed) is proven separately, against real Postgres, in
// internal/framework/persistence.
type RequestGateway struct {
	mu       sync.Mutex
	prefixes map[string]string
	requests map[string]port.CreateRequestInput
	numbers  map[string][]port.RequestNumberInput
	excluded map[string][]port.ExcludedNumberInput
	clients  map[string]port.ClientInput

	FailCreate error
}

// NewRequestGateway returns an empty double, ready to use.
func NewRequestGateway() *RequestGateway {
	return &RequestGateway{
		prefixes: map[string]string{},
		requests: map[string]port.CreateRequestInput{},
		numbers:  map[string][]port.RequestNumberInput{},
		excluded: map[string][]port.ExcludedNumberInput{},
		clients:  map[string]port.ClientInput{},
	}
}

// SeedPrefix registers an operator's routing prefix, read back by
// RoutingPrefix.
func (g *RequestGateway) SeedPrefix(operatorID, prefix string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.prefixes[operatorID] = prefix
}

func (g *RequestGateway) RoutingPrefix(_ context.Context, operatorID string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.prefixes[operatorID]
	if !ok {
		return "", errors.New("opérateur inconnu")
	}
	return p, nil
}

func (g *RequestGateway) Create(_ context.Context, in port.CreateRequestInput) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailCreate != nil {
		return g.FailCreate
	}
	g.requests[in.ID] = in
	return nil
}

func (g *RequestGateway) AddNumber(_ context.Context, in port.RequestNumberInput) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.numbers[in.RequestID] = append(g.numbers[in.RequestID], in)
	return nil
}

func (g *RequestGateway) AddExcludedNumber(_ context.Context, in port.ExcludedNumberInput) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.excluded[in.RequestID] = append(g.excluded[in.RequestID], in)
	return nil
}

func (g *RequestGateway) AddClient(_ context.Context, in port.ClientInput) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clients[in.RequestID] = in
	return nil
}

func (g *RequestGateway) Get(_ context.Context, id string) (port.RequestView, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	req, ok := g.requests[id]
	if !ok {
		return port.RequestView{}, false, nil
	}
	view := port.RequestView{
		ID: req.ID, MSISDN: req.MSISDN,
		SubscriberType: req.SubscriberType, RequestType: req.RequestType,
		Status: "EN_COURS", CurrentStep: "ACCEPTATION", CurrentStepStatus: "EN_COURS",
		SourceOperatorID: req.SourceOperatorID, SourceOperatorName: req.SourceOperatorID,
		RecipientOperatorID: req.RecipientOperatorID, RecipientOperatorName: req.RecipientOperatorID,
		RequestDate: req.RequestDate, Processus: req.Processus, RoutingInfo: req.RoutingInfo,
	}
	if c, ok := g.clients[id]; ok {
		view.Client = &port.ClientView{
			LastName: c.LastName, FirstName: c.FirstName,
			BirthPlace: c.BirthPlace, IDType: c.IDType, IDNumber: c.IDNumber,
		}
	}
	return view, true, nil
}

// Numbers exposes the retained numbers recorded for a request id — tests use
// it to assert what a fleet actually wrote.
func (g *RequestGateway) Numbers(id string) []port.RequestNumberInput {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]port.RequestNumberInput(nil), g.numbers[id]...)
}

// Excluded exposes the excluded numbers recorded for a request id.
func (g *RequestGateway) Excluded(id string) []port.ExcludedNumberInput {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]port.ExcludedNumberInput(nil), g.excluded[id]...)
}

// RequestCount reports how many requests were actually created — tests use
// it to assert that a rejected creation left nothing behind.
func (g *RequestGateway) RequestCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.requests)
}
