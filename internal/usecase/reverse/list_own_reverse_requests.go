package reverse

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListOwnReverseRequestsBoundary is the interface a controller drives for
// GET /reverse-requests/mes-demandes.
type ListOwnReverseRequestsBoundary interface {
	Execute(ctx context.Context, operatorID string, page, size int) ([]port.ReverseView, *entity.Fault)
}

// ListOwnReverseRequestsInteractor implements ListOwnReverseRequestsBoundary.
type ListOwnReverseRequestsInteractor struct {
	reverse port.ReverseGateway
}

// NewListOwnReverseRequests wires an interactor against its dependency.
func NewListOwnReverseRequests(reverse port.ReverseGateway) *ListOwnReverseRequestsInteractor {
	return &ListOwnReverseRequestsInteractor{reverse: reverse}
}

// Execute reproduces the deleted internal/api/reverse.go's
// getMesReverseRequests: every reverse request of the caller, every status,
// paginated (page, size — defaults 0 and 20, unlike the ten demande
// queues).
func (i *ListOwnReverseRequestsInteractor) Execute(
	ctx context.Context, operatorID string, page, size int,
) ([]port.ReverseView, *entity.Fault) {
	ids, err := i.reverse.Own(ctx, operatorID, page, size)
	if err != nil {
		return nil, entity.InternalError("reading the reverse requests")
	}

	out := make([]port.ReverseView, 0, len(ids))
	for _, id := range ids {
		view, found, err := i.reverse.Get(ctx, id)
		if err != nil || !found {
			return nil, entity.InternalError("reading the reverse request")
		}
		out = append(out, view)
	}
	return out, nil
}
