package auth

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// DescribeCallerInput carries the caller the authentication middleware has
// already resolved — GET /api/authenticate's whole business is confirming
// that resolution succeeded, so there is nothing else to bind.
type DescribeCallerInput struct {
	Caller entity.Caller
}

// DescribeCallerOutput is a pure projection of entity.Caller.
type DescribeCallerOutput struct {
	UserID       string
	Username     string
	OperatorID   string
	OperatorName string
}

// DescribeCallerBoundary is the interface a controller drives; the
// counterpart of AuthenticateBoundary above.
type DescribeCallerBoundary interface {
	Execute(context.Context, DescribeCallerInput) DescribeCallerOutput
}

// DescribeCallerInteractor implements DescribeCallerBoundary. By the time it
// runs, the authentication middleware has already verified the token and
// resolved its caller — Execute cannot fail, and needs no
// port.UserGateway: it only projects the entity.Caller it is handed.
type DescribeCallerInteractor struct{}

// NewDescribeCaller returns a ready-to-use interactor. It takes no
// dependency: unlike AuthenticateInteractor, it never resolves anything
// itself.
func NewDescribeCaller() *DescribeCallerInteractor { return &DescribeCallerInteractor{} }

func (i *DescribeCallerInteractor) Execute(_ context.Context, in DescribeCallerInput) DescribeCallerOutput {
	return DescribeCallerOutput{
		UserID:       in.Caller.UserID,
		Username:     in.Caller.Username,
		OperatorID:   in.Caller.OperatorID,
		OperatorName: in.Caller.OperatorName,
	}
}
