package db

import (
	"context"
	"errors"
	"time"

	"encore.dev/beta/errs"

	"encore.dev/storage/sqldb"

	"encore.dev/types/uuid"
)

var (
	TransferDB = sqldb.Named("transfer")
)

type TransferProgress string

const (
	TransferProgressInitiated                  TransferProgress = "initiated"
	TransferProgressInProcess                  TransferProgress = "in_process"
	TransferProgressSettled                    TransferProgress = "settled"
	TransferProgressFailedOnLedgerSettlement   TransferProgress = "failed_ledger_settlement"
	TransferProgressFailedOnLedgerTimeout      TransferProgress = "failed_ledger_timeout"
	TransferProgressFailedOnExternalDB         TransferProgress = "failed_external_db"
	TransferProgressFailedOnLedgerCancellation TransferProgress = "failed_ledger_cancellation"
)

type TransferResponse struct {
	ID               uuid.UUID `sql:"id"`
	DebitAccountID   uint64    `sql:"debit_account_id"`
	CreditAccountID  uint64    `sql:"credit_account_id"`
	Amount           uint64    `sql:"amount"`
	CreatedAt        time.Time `sql:"created_at"`
	TransferProgress string    `sql:"transfer_progress"`
}

func GetTransaction(ctx context.Context, customerAccount uint64, amount uint64, progress TransferProgress, forUpdate bool, tx *sqldb.Tx) (*TransferResponse, error) {
	var transfer TransferResponse
	query := `
		SELECT id, debit_account_id, credit_account_id, amount, created_at, transfer_progress FROM transfers
		WHERE debit_account_id = $1 AND amount = $2 AND transfer_progress = $3
		ORDER BY created_at ASC
		LIMIT 1`

	if forUpdate {
		query += " FOR UPDATE"
	}

	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, customerAccount, amount, progress).
			Scan(&transfer.ID, &transfer.DebitAccountID, &transfer.CreditAccountID, &transfer.Amount, &transfer.CreatedAt, &transfer.TransferProgress)
	} else {
		err = TransferDB.QueryRow(ctx, query, customerAccount, amount, progress).
			Scan(&transfer.ID, &transfer.DebitAccountID, &transfer.CreditAccountID, &transfer.Amount, &transfer.CreatedAt, &transfer.TransferProgress)
	}

	switch {
	case errors.Is(err, sqldb.ErrNoRows):
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "no transfer found for given authorization",
		}
	case err != nil:
		return nil, err
	}

	return &transfer, err
}

type TransferReq struct {
	ID              uuid.UUID
	DebitAccountID  uint64
	CreditAccountID uint64
	Amount          uint64
	Progress        TransferProgress
}

// InsertNewTransfer inserts a transfer into the database idempotently on id
func InsertNewTransfer(req *TransferReq) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := TransferDB.Exec(ctx, `
		INSERT INTO transfers (id, debit_account_id, credit_account_id, amount)
		    VALUES ($1, $2, $3, $4)
		    ON CONFLICT (id) DO NOTHING`, req.ID, req.DebitAccountID, req.CreditAccountID, req.Amount)
	return err
}

// InsertNewTransferWithProgress inserts a transfer into the database idempotently on id withj given progress
func InsertNewTransferWithProgress(req *TransferReq) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := TransferDB.Exec(ctx, `
		INSERT INTO transfers (id, debit_account_id, credit_account_id, amount, transfer_progress)
		    VALUES ($1, $2, $3, $4, $5)
		    ON CONFLICT (id) DO NOTHING`, req.ID, req.DebitAccountID, req.CreditAccountID, req.Amount, req.Progress)
	return err
}

func UpdateTransferProgress(id uuid.UUID, progress TransferProgress, tx *sqldb.Tx) error {
	dbCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if tx != nil {
		_, err := tx.Exec(dbCtx, `
			update transfers set transfer_progress = $1 WHERE id = $2`, progress, id)
		return err
	}
	_, err := TransferDB.Exec(dbCtx, `
		update transfers set transfer_progress = $1 WHERE id = $2`, progress, id)

	return err
}
