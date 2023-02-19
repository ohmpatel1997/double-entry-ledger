package ledger

import (
	"context"

	"encore.dev/beta/errs"

	tb "github.com/tigerbeetledb/tigerbeetle-go"
	tb_types "github.com/tigerbeetledb/tigerbeetle-go/pkg/types"
)

// encore:service
type APIService struct {
	Ledger *ledgerService
}

func initAPIService() (*APIService, error) {
	tbClient, err := tb.NewClient(0, []string{"3000"}, 1)
	if err != nil {
		return nil, err
	}

	return &APIService{
		Ledger: NewLedgerService(tbClient),
	}, nil
}

func (api *APIService) Shutdown(force context.Context) {
	api.Ledger.TB.Close()
}

//encore:api public method=POST path=/accounts
func (api *APIService) Account(ctx context.Context, req *AccountReq) error {
	resp, err := api.Ledger.CreateAccount(req.ID)
	if err != nil {
		return err
	}

	switch resp.Result {
	case tb_types.AccountOK:
		return nil
	default:
		return &errs.Error{
			Code:    errs.Internal,
			Message: resp.Result.String(),
		}
	}
}

type AccountReq struct {
	ID uint64 `json:"id"`
}

//encore:api public method=GET path=/accounts/:id/balance
func (api *APIService) Balance(ctx context.Context, id string) (*BalanceResponse, error) {
	return new(BalanceResponse), nil
}

type BalanceResponse struct {
	AvailableBalance float64 `json:"available_balance"`
	ReservedBalance  float64 `json:"reserved_balance"`
}

//encore:api public method=POST path=/accounts/:id/authorize
func (api *APIService) Authorize(ctx context.Context, id string, req *AuthorizeRequest) (*AuthorizeResponse, error) {
	return new(AuthorizeResponse), nil
}

type AuthorizeRequest struct {
	Amount float64 `json:"amount"`
}

type AuthorizeResponse struct {
	AvailableBalance float64 `json:"available_balance"`
	ReservedBalance  float64 `json:"reserved_balance"`
}

//encore:api public POST path=/accounts/:id/present
func (api *APIService) Present(ctx context.Context, id string, req *PresentRequest) (*PresentResponse, error) {
	return new(PresentResponse), nil
}

type PresentRequest struct {
	Amount float64 `json:"amount"`
}

type PresentResponse struct {
	Ok int64 `json:"ok"`
}
