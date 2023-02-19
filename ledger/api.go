package ledger

import (
	"context"
)

//encore:api public path=/accounts/:id/balance
func Balance(ctx context.Context, id string) (*BalanceResponse, error) {
	return new(BalanceResponse), nil
}

type BalanceResponse struct {
	AvailableBalance float64 `json:"available_balance"`
	ReservedBalance  float64 `json:"reserved_balance"`
}

//encore:api public path=/accounts/:id/authorize
func Authorize(ctx context.Context, id string, req *AuthorizeRequest) (*AuthorizeResponse, error) {
	return new(AuthorizeResponse), nil
}

type AuthorizeRequest struct {
	Amount float64 `json:"amount"`
}

type AuthorizeResponse struct {
	AvailableBalance float64 `json:"available_balance"`
	ReservedBalance  float64 `json:"reserved_balance"`
}

//encore:api public path=/accounts/:id/present
func Present(ctx context.Context, id string, req *PresentRequest) (*PresentResponse, error) {
	return new(PresentResponse), nil
}

type PresentRequest struct {
	Amount float64 `json:"amount"`
}

type PresentResponse struct {
	Ok int64 `json:"ok"`
}
