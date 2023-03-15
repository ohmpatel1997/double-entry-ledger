package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"encore.dev/beta/errs"

	"github.com/ohmpatel1997/pave-coding-challenge-simon/ledger"
	"github.com/ohmpatel1997/pave-coding-challenge-simon/transfer"
	tb "github.com/tigerbeetledb/tigerbeetle-go"
)

// encore:service
type APIService struct {
	Ledger *ledger.Service
}

func initAPIService() (*APIService, error) {
	tbClient, err := tb.NewClient(0, []string{"2000"}, 1)
	if err != nil {
		return nil, errs.Wrap(errors.New("error connecting to db"), err.Error())
	}

	return &APIService{
		Ledger: ledger.NewLedgerService(tbClient),
	}, nil
}

func (api *APIService) Shutdown(force context.Context) {
	api.Ledger.TB.Close()
}

//encore:api public method=POST path=/accounts
func (api *APIService) Account(ctx context.Context, req *AccountReq) error {
	err := api.Ledger.CreateAccount(req.ID, req.AccountType)
	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error creating account: %s", err.Error()),
		}
	}

	return nil
}

type AccountReq struct {
	ID          uint64 `json:"id"`
	AccountType uint16 `json:"account_type"`
}

//encore:api public method=GET path=/accounts/:id
func (api *APIService) GetAccount(ctx context.Context, id uint64) (*AccountResp, error) {
	resp, err := api.Ledger.GetAccount(id)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error creating account: %s", err.Error()),
		}
	}

	return &AccountResp{
		ID:             resp.ID.String(),
		Ledger:         resp.Ledger,
		Code:           resp.Code,
		Flags:          resp.Flags,
		DebitsPending:  resp.DebitsPending,
		DebitsPosted:   resp.DebitsPosted,
		CreditsPending: resp.CreditsPending,
		CreditsPosted:  resp.CreditsPosted,
		Timestamp:      time.Unix(0, int64(resp.Timestamp)),
	}, nil
}

type AccountResp struct {
	ID             string
	Ledger         uint32
	Code           uint16
	Flags          uint16
	DebitsPending  uint64
	DebitsPosted   uint64
	CreditsPending uint64
	CreditsPosted  uint64
	Timestamp      time.Time
}

//encore:api public method=GET path=/accounts/:id/balance
func (api *APIService) Balance(ctx context.Context, id uint64) (*BalanceResponse, error) {
	acc, err := api.Ledger.GetAccount(id)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error getting account: %s", err.Error()),
		}
	}

	return &BalanceResponse{
		AvailableBalance: "$" + strconv.FormatFloat(float64(acc.CreditsPosted-acc.DebitsPosted-acc.DebitsPending)/100, 'f', 2, 64),
		ReservedBalance:  "$" + strconv.FormatFloat(float64(acc.DebitsPending)/100, 'f', 2, 64),
	}, nil
}

type BalanceResponse struct {
	AvailableBalance string `json:"available_balance"`
	ReservedBalance  string `json:"reserved_balance"`
}

//encore:api public method=POST path=/accounts/:id/authorize
func (api *APIService) Authorize(ctx context.Context, id uint64, req *AuthorizeRequest) error {
	acc, err := api.Ledger.GetAccount(id)
	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error getting account: %s", err.Error()),
		}
	}

	// check if the account has enough balance
	if acc.CreditsPosted-(acc.DebitsPosted+acc.DebitsPending) < uint64(req.Amount*100) {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("insufficient balance"),
		}
	}

	err = transfer.Transfer(ctx, &transfer.Request{
		CustomerAccount: id,
		TxnType:         transfer.TransactionTypeCreditCardAuth,
		Amount:          uint64(req.Amount * 100),
	})

	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error authorizing: %s", err.Error()),
		}
	}
	return nil
}

type AuthorizeRequest struct {
	Amount float64 `json:"amount"`
}

//encore:api public method=POST path=/accounts/:id/present
func (api *APIService) Present(ctx context.Context, id uint64, req *PresentRequest) error {
	// check if the account exists
	_, err := api.Ledger.GetAccount(id)
	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error getting account: %s", err.Error()),
		}
	}

	// check if there is a pending auth transfer
	// just an extra guard, actual matching of presentment and auth is done in the transfer service
	_, err = transfer.GetAuthTransferForPresentment(ctx, &transfer.PresentmentRequest{
		Account: id,
		Amount:  uint64(req.Amount * 100),
	})
	if err != nil {
		return err
	}

	err = transfer.Transfer(ctx, &transfer.Request{
		CustomerAccount: id,
		TxnType:         transfer.TransactionTypeCreditCardPresent,
		Amount:          uint64(req.Amount * 100),
	})

	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error presenting: %s", err.Error()),
		}
	}
	return nil
}

type PresentRequest struct {
	Amount float64 `json:"amount"`
}

type PresentResponse struct {
	Ok int64 `json:"ok"`
}
type TransferRequest struct {
	FromAccountID uint64  `json:"from_account_id"`
	ToAccountID   uint64  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
}

//encore:api public method=POST path=/internal/transfers
func (api *APIService) Transfer(ctx context.Context, req *TransferRequest) error {
	return api.Ledger.Transfer(&ledger.TransferReq{
		DebitAccountID:  req.ToAccountID, // user account debit increases the bank asset, so it's a credit for bank
		CreditAccountID: req.FromAccountID,
		Amount:          uint64(req.Amount * 100), // convert to cents and take the floor
	})
}
