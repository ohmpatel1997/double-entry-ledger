package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"encore.dev/beta/errs"
	"go.temporal.io/sdk/workflow"

	"github.com/ohmpatel1997/pave-coding-challenge-simon/transfer/db"
)

func signalActivity(ctx workflow.Context, req *PaymentDetails) error {
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.TransferDB.Begin(dbCtx)
	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: "internal error",
		}
	}

	// get a initiated transfer, get a lock, so not other workflow can pick it up
	transfer, err := db.GetTransaction(dbCtx, req.SourceAccount, req.Amount, db.TransferProgressInitiated, true, tx)
	var encoreErr *errs.Error
	switch {
	// in case if there is another workflow that has already settled the transaction
	case err != nil && errors.As(err, &encoreErr) && encoreErr.Code == errs.NotFound:
		tx.Rollback()
		return nil
	case err != nil:
		tx.Rollback()
		return err
	}

	// signal the auth workflow to settle transaction
	signal := workflow.SignalExternalWorkflow(ctx, transfer.ID.String(), "", fmt.Sprintf("presentment-%s", transfer.ID.String()), PresentmentSignal{ID: transfer.ID.String()})
	if err = signal.Get(ctx, nil); err != nil {
		tx.Rollback()
		return err
	}

	// update the transfer progress to in progress so that the another workflow doesn't pick it up
	err = db.UpdateTransferProgress(dbCtx, transfer.ID, db.TransferProgressInProcess, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
