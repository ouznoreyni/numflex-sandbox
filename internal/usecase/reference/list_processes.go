package reference

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListProcessesBoundary is the interface a controller drives.
type ListProcessesBoundary interface {
	Execute(context.Context) ([]entity.Process, *entity.Fault)
}

// ListProcessesInteractor implements ListProcessesBoundary.
type ListProcessesInteractor struct {
	gateway port.ReferenceGateway
}

// NewListProcesses wires an interactor against the given gateway.
func NewListProcesses(g port.ReferenceGateway) *ListProcessesInteractor {
	return &ListProcessesInteractor{gateway: g}
}

func (i *ListProcessesInteractor) Execute(ctx context.Context) ([]entity.Process, *entity.Fault) {
	out, err := i.gateway.Processes(ctx)
	if err != nil {
		return nil, entity.InternalError("lecture des processus")
	}
	return out, nil
}
