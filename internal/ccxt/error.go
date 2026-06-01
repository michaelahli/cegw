package ccxt

import (
	ccxt "github.com/ccxt/ccxt/go/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func MapError(err error) error {
	if err == nil {
		return nil
	}

	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected error: %v", err)
	}

	switch ccxtErr.Type {
	case ccxt.InsufficientFundsErrType:
		return status.Error(codes.FailedPrecondition, "insufficient balance to place order")
	case ccxt.InvalidOrderErrType:
		return status.Errorf(codes.InvalidArgument, "invalid order: %s", ccxtErr.Message)
	case ccxt.AuthenticationErrorErrType:
		return status.Error(codes.Unauthenticated, "invalid credentials")
	case ccxt.NetworkErrorErrType, ccxt.ExchangeNotAvailableErrType, ccxt.RequestTimeoutErrType:
		return status.Error(codes.Unavailable, "exchange temporarily unavailable")
	case ccxt.RateLimitExceededErrType:
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	case ccxt.BadSymbolErrType:
		return status.Errorf(codes.NotFound, "unknown symbol: %s", ccxtErr.Message)
	default:
		return status.Errorf(codes.Internal, "exchange error: %v", ccxtErr.Type)
	}
}
