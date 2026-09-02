package inmemory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// RequestGateway is a map-backed port.RequestGateway, for use case unit
// tests that must not touch a database. FailCreate, when set, makes Create
// return it instead of writing — the seam a use-case-level test uses to
// exercise the "insert fails" branch without a real transaction; the
// transaction *guarantee* itself (that a failed Create really leaves the
// OTP unconsumed) is proven separately, against real Postgres, in
// internal/framework/persistence.
//
// portingRequests, comments and the two number maps back Task 14's six
// acceptance methods (ByID, SetComment, NumberBelongs, RejectNumber,
// HasActiveNumber, Reject). They are a separate state machine from
// requests/numbers above, seeded directly with Seed/SeedNumbers rather than
// through Create/AddNumber: an acceptance test starts from a request
// already sitting at ACCEPTATION, it does not create one first.
type RequestGateway struct {
	mu       sync.Mutex
	prefixes map[string]string
	requests map[string]port.CreateRequestInput
	numbers  map[string][]port.RequestNumberInput
	excluded map[string][]port.ExcludedNumberInput
	clients  map[string]port.ClientInput

	portingRequests  map[string]entity.PortingRequest
	comments         map[string]string
	rejectionReasons map[string]string            // requestID -> motifRejetId, written by Reject
	numberStatus     map[string]map[string]string // requestID -> msisdn -> status
	numberMotif      map[string]map[string]string // requestID -> msisdn -> motifRejetId

	FailCreate       error
	FailReject       error // fails Reject, the seam acceptance's atomicity test uses
	FailRejectNumber error // fails RejectNumber, same seam for the fleet path
	FailCancel       error // fails Cancel, the same seam for porting.CancelRequest
	FailSetComment   error // fails SetComment, the same seam for porting.ProcessStep
}

// NewRequestGateway returns an empty double, ready to use.
func NewRequestGateway() *RequestGateway {
	return &RequestGateway{
		prefixes: map[string]string{},
		requests: map[string]port.CreateRequestInput{},
		numbers:  map[string][]port.RequestNumberInput{},
		excluded: map[string][]port.ExcludedNumberInput{},
		clients:  map[string]port.ClientInput{},

		portingRequests:  map[string]entity.PortingRequest{},
		comments:         map[string]string{},
		rejectionReasons: map[string]string{},
		numberStatus:     map[string]map[string]string{},
		numberMotif:      map[string]map[string]string{},
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
		// Not created through Create/AddNumber: fall back to a request
		// Seed()ed directly for an acceptance test, which starts from a
		// request already sitting at ACCEPTATION rather than creating one.
		pr, ok := g.portingRequests[id]
		if !ok {
			return port.RequestView{}, false, nil
		}
		return port.RequestView{
			ID: pr.ID, MSISDN: pr.MSISDN,
			SubscriberType: string(pr.SubscriberType), RequestType: string(pr.RequestType),
			Status: string(pr.Status), CurrentStep: string(pr.CurrentStep),
			CurrentStepStatus: string(pr.CurrentStepStatus),
			SourceOperatorID:  pr.SourceOperatorID, SourceOperatorName: pr.SourceOperatorID,
			RecipientOperatorID: pr.RecipientOperatorID, RecipientOperatorName: pr.RecipientOperatorID,
		}, true, nil
	}
	view := port.RequestView{
		ID: req.ID, MSISDN: req.MSISDN,
		SubscriberType: req.SubscriberType, RequestType: req.RequestType,
		Status: "EN_COURS", CurrentStep: "ACCEPTATION", CurrentStepStatus: "EN_COURS",
		SourceOperatorID: req.SourceOperatorID, SourceOperatorName: req.SourceOperatorID,
		RecipientOperatorID: req.RecipientOperatorID, RecipientOperatorName: req.RecipientOperatorID,
		RequestDate: req.RequestDate, Process: req.Process, RoutingInfo: req.RoutingInfo,
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

// --- Acceptance (Task 14) ---------------------------------------------------

// Seed registers a request's authorization-relevant shape, read back by
// ByID — an acceptance test's starting point, independent of Create.
func (g *RequestGateway) Seed(pr entity.PortingRequest) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.portingRequests[pr.ID] = pr
}

// SeedNumbers registers requestID's fleet members, all initially EN_COURS —
// the fleet-rejection tests' starting point, independent of AddNumber.
func (g *RequestGateway) SeedNumbers(requestID string, msisdns ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	statuses := map[string]string{}
	for _, m := range msisdns {
		statuses[m] = "EN_COURS"
	}
	g.numberStatus[requestID] = statuses
	g.numberMotif[requestID] = map[string]string{}
}

func (g *RequestGateway) ByID(_ context.Context, id string) (entity.PortingRequest, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	pr, ok := g.portingRequests[id]
	return pr, ok, nil
}

func (g *RequestGateway) SetComment(_ context.Context, id, comment string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailSetComment != nil {
		return g.FailSetComment
	}
	g.comments[id] = comment
	return nil
}

func (g *RequestGateway) NumberBelongs(_ context.Context, requestID, msisdn string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.numberStatus[requestID][msisdn]
	return ok, nil
}

func (g *RequestGateway) RejectNumber(_ context.Context, requestID, msisdn, rejectionReasonID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailRejectNumber != nil {
		return g.FailRejectNumber
	}
	g.numberStatus[requestID][msisdn] = "REJETE"
	g.numberMotif[requestID][msisdn] = rejectionReasonID
	return nil
}

func (g *RequestGateway) HasActiveNumber(_ context.Context, requestID string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, status := range g.numberStatus[requestID] {
		if status != "REJETE" {
			return true, nil
		}
	}
	return false, nil
}

func (g *RequestGateway) Reject(_ context.Context, requestID, _, rejectionReasonID,
	comment string, _ time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailReject != nil {
		return g.FailReject
	}
	pr := g.portingRequests[requestID]
	pr.Status = entity.RequestRejected
	pr.CurrentStepStatus = entity.StepCompleted
	g.portingRequests[requestID] = pr
	g.comments[requestID] = comment
	g.rejectionReasons[requestID] = rejectionReasonID
	return nil
}

// Cancel — moved verbatim in shape from Reject above (Task 15): a
// cancellation has no rejection reason and no comment, but does move
// the request to RequestCancelled rather than RequestRejected. expectedStep
// is accepted for interface conformance with the real gateway's step guard
// (Task 17b) but not checked here: this double has no concurrent writer to
// race against, and the guard itself is proven only where it matters, in
// internal/framework/engine's own integration test against real Postgres.
func (g *RequestGateway) Cancel(_ context.Context, requestID, _ string, _ entity.Step, _ time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailCancel != nil {
		return g.FailCancel
	}
	pr := g.portingRequests[requestID]
	pr.Status = entity.RequestCancelled
	pr.CurrentStepStatus = entity.StepCompleted
	g.portingRequests[requestID] = pr
	return nil
}

// Status exposes a seeded/rejected request's current status — tests use it
// to assert Reject actually ran.
func (g *RequestGateway) Status(id string) entity.RequestStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.portingRequests[id].Status
}

// Comment exposes the comment written by SetComment or Reject.
func (g *RequestGateway) Comment(id string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.comments[id]
}

// NumberStatus exposes one fleet member's current status — EN_COURS unless
// RejectNumber (or a full Reject) marked it REJETE.
func (g *RequestGateway) NumberStatus(requestID, msisdn string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.numberStatus[requestID][msisdn]
}

// RejectionReason exposes the motifRejetId written by Reject.
func (g *RequestGateway) RejectionReason(id string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rejectionReasons[id]
}

// NumberRejectionReason exposes the motifRejetId written by RejectNumber
// for one fleet member.
func (g *RequestGateway) NumberRejectionReason(requestID, msisdn string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.numberMotif[requestID][msisdn]
}

// --- Platform engine (Task 17) ----------------------------------------------
//
// The methods below back internal/usecase/platform, exercised at the
// integration level (internal/framework/engine, against real Postgres); no
// use-case-level unit test drives them today. They operate on the same
// portingRequests map ByID/Seed already use, kept just correct enough to
// satisfy
// port.RequestGateway rather than mirroring every SQL side effect (routage,
// registry transfer) that has no equivalent map here.

func (g *RequestGateway) LockForTransition(_ context.Context, id string) (entity.PortingRequest, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	pr, ok := g.portingRequests[id]
	return pr, ok, nil
}

func (g *RequestGateway) CloseCurrentStep(_ context.Context, id string, closedStatus entity.StepStatus, _ string, _ time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pr := g.portingRequests[id]
	pr.CurrentStepStatus = closedStatus
	g.portingRequests[id] = pr
	return nil
}

func (g *RequestGateway) CompleteRequest(_ context.Context, id string, closedStatus entity.StepStatus, _ time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pr := g.portingRequests[id]
	pr.Status = entity.RequestCompleted
	pr.CurrentStepStatus = closedStatus
	pr.PendingTransition = false
	g.portingRequests[id] = pr
	return nil
}

func (g *RequestGateway) AdvanceStep(_ context.Context, id string, next entity.Step, _ time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pr := g.portingRequests[id]
	pr.CurrentStep = next
	pr.CurrentStepStatus = entity.StepInProgress
	pr.PendingTransition = false
	g.portingRequests[id] = pr
	return nil
}

func (g *RequestGateway) TransferToRegistry(_ context.Context, _, _ string) error { return nil }

func (g *RequestGateway) ApplyRouting(_ context.Context, _, _, _ string) error { return nil }

func (g *RequestGateway) ApplyEndOfRequestRestitution(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (g *RequestGateway) ScheduleTransitionAt(_ context.Context, id string, _ float64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pr := g.portingRequests[id]
	pr.PendingTransition = true
	g.portingRequests[id] = pr
	return nil
}

func (g *RequestGateway) DueConvergences(_ context.Context) ([]string, error) {
	return nil, nil
}

func (g *RequestGateway) OverdueSteps(_ context.Context, _ float64, _ time.Time) ([]string, error) {
	return nil, nil
}

func (g *RequestGateway) CreateAtConfirmation(_ context.Context, in port.CreateRequestInput) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailCreate != nil {
		return g.FailCreate
	}
	g.requests[in.ID] = in
	g.portingRequests[in.ID] = entity.PortingRequest{
		ID: in.ID, MSISDN: in.MSISDN,
		RequestType: entity.RequestType(in.RequestType), SubscriberType: entity.SubscriberType(in.SubscriberType),
		Status: entity.RequestInProgress, CurrentStep: entity.StepConfirmation, CurrentStepStatus: entity.StepInProgress,
		SourceOperatorID: in.SourceOperatorID, RecipientOperatorID: in.RecipientOperatorID,
		CreatorOperatorID: in.CreatorOperatorID,
	}
	return nil
}

func (g *RequestGateway) PendingReverseCompletions(_ context.Context) ([]port.PendingReverseCompletion, error) {
	return nil, nil
}
