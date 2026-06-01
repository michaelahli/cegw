package ccxt

import (
	"testing"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name:     "nil error",
			err:      nil,
			wantCode: codes.OK,
		},
		{
			name:     "insufficient funds",
			err:      &ccxt.Error{Type: ccxt.InsufficientFundsErrType},
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "invalid order",
			err:      &ccxt.Error{Type: ccxt.InvalidOrderErrType},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "authentication error",
			err:      &ccxt.Error{Type: ccxt.AuthenticationErrorErrType},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "network error",
			err:      &ccxt.Error{Type: ccxt.NetworkErrorErrType},
			wantCode: codes.Unavailable,
		},
		{
			name:     "rate limit exceeded",
			err:      &ccxt.Error{Type: ccxt.RateLimitExceededErrType},
			wantCode: codes.ResourceExhausted,
		},
		{
			name:     "bad symbol",
			err:      &ccxt.Error{Type: ccxt.BadSymbolErrType},
			wantCode: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapError(tt.err)
			if tt.err == nil {
				if err != nil {
					t.Errorf("MapError() = %v, want nil", err)
				}
				return
			}

			st, ok := status.FromError(err)
			if !ok {
				t.Errorf("MapError() did not return a status error")
				return
			}

			if st.Code() != tt.wantCode {
				t.Errorf("MapError() code = %v, want %v", st.Code(), tt.wantCode)
			}
		})
	}
}
