package auth_test

import (
	"context"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/auth"
)

// TestDescribeCallerProjectsCaller pins that DescribeCaller is a pure
// projection: every field of the resolved entity.Caller comes back
// unchanged, and no gateway is ever touched (NewDescribeCaller takes none).
func TestDescribeCallerProjectsCaller(t *testing.T) {
	i := auth.NewDescribeCaller()

	out := i.Execute(context.Background(), auth.DescribeCallerInput{
		Caller: entity.Caller{
			UserID: "u1", Username: "yas", OperatorID: "op-yas", OperatorName: "YAS",
		},
	})

	if out.UserID != "u1" || out.Username != "yas" ||
		out.OperatorID != "op-yas" || out.OperatorName != "YAS" {
		t.Fatalf("got %+v, want the caller's fields projected unchanged", out)
	}
}
