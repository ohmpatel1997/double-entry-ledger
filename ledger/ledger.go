package ledger

import (
	"errors"
	"fmt"
	"math/rand"

	"encore.dev/beta/errs"
	"go.temporal.io/sdk/temporal"

	"encore.dev/types/uuid"

	tb "github.com/tigerbeetledb/tigerbeetle-go"
	tb_types "github.com/tigerbeetledb/tigerbeetle-go/pkg/types"
)

type Service struct {
	TB tb.Client
}

func NewLedgerService(tb tb.Client) *Service {
	return &Service{
		TB: tb,
	}
}

func (l *Service) CreateAccount(id uint64, accType uint16) error {
	idUint128, err := tb_types.HexStringToUint128(fmt.Sprintf("%d", id))
	if err != nil {
		return errs.Wrap(err, "error parsing the id")
	}

	var res []tb_types.AccountEventResult
	switch accType {
	case 1:
		// create a customer account
		res, err = l.TB.CreateAccounts([]tb_types.Account{
			{
				ID:     idUint128,
				Ledger: uint32(1), // for now constant
				Code:   accType,
				Flags: tb_types.AccountFlags{
					DebitsMustNotExceedCredits: true,
				}.ToUint16(),
			},
		})
	case 2:
		// create a bank settlement account
		// create a customer account
		res, err = l.TB.CreateAccounts([]tb_types.Account{
			{
				ID:     idUint128,
				Ledger: uint32(1), // for now constant
				Code:   accType,
				Flags: tb_types.AccountFlags{
					CreditsMustNotExceedDebits: true,
				}.ToUint16(),
			},
		})
	}

	if err != nil {
		return errs.Wrap(err, "error creating account")
	}

	for _, r := range res {
		switch r.Result {
		case tb_types.AccountExists:
			return nil
		default:
			return &errs.Error{
				Code:    errs.Internal,
				Message: fmt.Sprintf("error creating account: %s", r.Result.String()),
			}
		}

	}

	return err
}

func (l *Service) GetAccount(id uint64) (*tb_types.Account, error) {
	idUint128, err := tb_types.HexStringToUint128(fmt.Sprintf("%d", id))
	if err != nil {
		return nil, errs.Wrap(err, "error parsing the id")
	}

	acc, err := l.TB.LookupAccounts([]tb_types.Uint128{
		idUint128,
	})

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: err.Error(),
		}
	}

	if len(acc) == 0 {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "account not found",
		}
	}

	return &acc[0], nil
}

type TransferReq struct {
	ID              uuid.UUID
	DebitAccountID  uint64
	CreditAccountID uint64
	Amount          uint64
}

func (l *Service) Transfer(transfer *TransferReq) error {
	debitIDUint128, err := tb_types.HexStringToUint128(fmt.Sprintf("%d", transfer.DebitAccountID))
	if err != nil {
		return errs.Wrap(err, "error parsing the id")
	}

	creditIDUint128, err := tb_types.HexStringToUint128(fmt.Sprintf("%d", transfer.CreditAccountID))
	if err != nil {
		return errs.Wrap(err, "error parsing the id")
	}

	var id tb_types.Uint128
	var found = false
	for tries := 0; tries < 5; tries++ {
		id, err = tb_types.HexStringToUint128(fmt.Sprintf("%d", rand.Int63()))
		if err != nil {
			return errs.Wrap(err, "error parsing the id")
		}

		existinTransfers, err := l.TB.LookupTransfers([]tb_types.Uint128{
			id,
		})
		if err != nil {
			return errs.Wrap(err, "error getting the transfer")
		}

		if len(existinTransfers) == 0 {
			found = true
			break
		}
	}

	if !found {
		return &errs.Error{
			Code:    errs.Internal,
			Message: "error creating transfer. pls retry again",
		}
	}

	newTransfer, err := l.TB.CreateTransfers([]tb_types.Transfer{
		{
			ID:              id,
			DebitAccountID:  debitIDUint128,
			CreditAccountID: creditIDUint128,
			Amount:          transfer.Amount,
			Ledger:          uint32(1),
			Code:            uint16(1),
		},
	})

	if err != nil {
		return errs.Wrap(err, "error creating the transfer")
	}

	for _, transfer := range newTransfer {
		return &errs.Error{
			Code:    errs.Internal,
			Message: fmt.Sprintf("error creating transfer: %s", transfer.Result.String()),
		}
	}

	return nil
}

func (l *Service) FreezeAmount(req *TransferReq) error {
	id := toU128(req.ID.Bytes())
	debitAccID, err := tb_types.HexStringToUint128(fmt.Sprintf("%d", req.DebitAccountID))
	if err != nil {
		return temporal.NewNonRetryableApplicationError("error parsing the debit account id", "invalid_id", errs.Wrap(err, "error parsing the debit account id"))
	}

	creditAccID, err := tb_types.HexStringToUint128(fmt.Sprintf("%d", req.CreditAccountID))
	if err != nil {
		return temporal.NewNonRetryableApplicationError("error parsing the credit account id", "invalid_id", errs.Wrap(err, "error parsing the credit account id"))
	}

	resp, err := l.TB.CreateTransfers([]tb_types.Transfer{
		{
			ID:              id,
			DebitAccountID:  debitAccID,
			CreditAccountID: creditAccID,
			Amount:          req.Amount,
			Flags: tb_types.TransferFlags{
				Pending: true,
			}.ToUint16(),
			Ledger: uint32(1), // for now constant
			Code:   uint16(1), // for now constant
		},
	})

	if err != nil {
		return errs.Wrap(err, "error creating the transfer")
	}

	for _, transfer := range resp {
		switch {
		case transfer.Result == tb_types.TransferOK,
			transfer.Result == tb_types.TransferExists:
			return nil
		default:
			return temporal.NewNonRetryableApplicationError(fmt.Sprintf("error creating transfer"), transfer.Result.String(), errors.New(transfer.Result.String()), nil)
		}
	}
	return nil
}

func (l *Service) SettleTransaction(pendingID uuid.UUID, newID uuid.UUID) error {
	parsedPendingID := toU128(pendingID.Bytes())

	parsedSettlementID := toU128(newID.Bytes())

	resp, err := l.TB.CreateTransfers([]tb_types.Transfer{
		{
			ID:        parsedSettlementID,
			PendingID: parsedPendingID,
			Flags: tb_types.TransferFlags{
				PostPendingTransfer: true,
			}.ToUint16(),
		},
	})

	if err != nil {
		return errs.Wrap(err, "error creating the transfer")
	}

	for _, transfer := range resp {
		switch transfer.Result {
		case tb_types.TransferOK,
			tb_types.TransferExists:
			return nil
		default:
			return temporal.NewNonRetryableApplicationError(fmt.Sprintf("error creating transfer"), "transfer already exist", errors.New(transfer.Result.String()), nil)
		}
	}

	return nil
}

func (l *Service) CancelTransaction(transactionID uuid.UUID, newID uuid.UUID) error {
	parseTransferID := toU128(transactionID.Bytes())

	transfer, err := l.TB.LookupTransfers([]tb_types.Uint128{
		parseTransferID,
	})

	if err != nil {
		return errs.Wrap(err, "error getting the transfer")
	}

	if len(transfer) == 0 {
		return temporal.NewNonRetryableApplicationError("transfer not found", "not found", errors.New("transfer not found"), nil)
	}

	pendingFlag := tb_types.TransferFlags{Pending: true}.ToUint16()
	if transfer[0].Flags != pendingFlag {
		return temporal.NewNonRetryableApplicationError("transfer is no more in pending state", "not pending", errors.New("transfer is not pending"), nil)
	}

	parsedNewID := toU128(newID.Bytes())
	if err != nil {
		return temporal.NewNonRetryableApplicationError("error parsing transfer id", "invalid_id", errs.Wrap(err, "error parsing the transfer id"))
	}

	resp, err := l.TB.CreateTransfers([]tb_types.Transfer{
		{
			ID:        parsedNewID,
			PendingID: parseTransferID,
			Flags: tb_types.TransferFlags{
				VoidPendingTransfer: true,
			}.ToUint16(),
		},
	})

	if err != nil {
		return errs.Wrap(err, "error creating the transfer")
	}

	for _, transfer := range resp {
		switch transfer.Result {
		case tb_types.TransferOK,
			tb_types.TransferExists:
			return nil
		default:
			return temporal.NewNonRetryableApplicationError(fmt.Sprintf("error creating transfer"), "transfer already exist", errors.New(transfer.Result.String()), nil)
		}
	}

	return nil
}

func toU128(value []byte) tb_types.Uint128 {
	var reqID [16]byte
	copy(reqID[:], value)
	return tb_types.BytesToUint128(reqID)
}
