package porting_test

import (
	"context"
	"errors"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

const (
	orangeID   = "operateur-orange"
	yasID      = "operateur-yas"
	expressoID = "operateur-expresso"
)

var errBoom = errors.New("échec simulé de la couche gateway")

func ctxCaller(operatorID string) context.Context {
	return port.WithCaller(context.Background(), entity.Caller{OperatorID: operatorID})
}

// fixture bundles the doubles a porting interactor test wires — the same
// shape internal/usecase/acceptance's own fixture already establishes.
type fixture struct {
	requests      *inmemory.RequestGateway
	operators     *inmemory.ReferenceGateway
	confirmations *inmemory.ConfirmationGateway
	uow           *inmemory.UnitOfWork
	engine        *inmemory.Engine
	clock         inmemory.FixedClock
}

func newFixture() *fixture {
	requests := inmemory.NewRequestGateway()
	confirmations := inmemory.NewConfirmationGateway()
	return &fixture{
		requests:      requests,
		operators:     inmemory.NewReferenceGateway(),
		confirmations: confirmations,
		uow: inmemory.NewUnitOfWork(port.Repositories{
			Requests: requests, Confirmations: confirmations,
		}),
		engine: inmemory.NewEngine(),
		clock:  inmemory.FixedClock{At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)},
	}
}

// seedRequest registers a request already sitting at DESACTIVATION,
// ORANGE (source) → YAS (recipient, and creator) — porting's tests start
// from a request already accepted, exactly as acceptance's own tests start
// from one already created. mutate lets a test move it to a different step
// or type before it is seeded.
func seedRequest(f *fixture, mutate ...func(*entity.PortingRequest)) entity.PortingRequest {
	pr := entity.PortingRequest{
		ID: "d1", RequestType: entity.RequestTypePorting, SubscriberType: entity.SubscriberIndividual,
		Status: entity.RequestInProgress, CurrentStep: entity.StepDeactivation,
		CurrentStepStatus:   entity.StepInProgress,
		SourceOperatorID:    orangeID,
		RecipientOperatorID: yasID,
		CreatorOperatorID:   yasID,
	}
	for _, m := range mutate {
		m(&pr)
	}
	f.requests.Seed(pr)
	return pr
}
