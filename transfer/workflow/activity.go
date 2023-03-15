package workflow

import (
	"context"
	"errors"
	"fmt"

	"encore.dev/beta/errs"

	"github.com/ohmpatel1997/pave-coding-challenge-simon/transfer/db"
)

func (s *Service) SignalActivity(ctx context.Context, req *PaymentDetails) error {
	tx, err := db.TransferDB.Begin(ctx)
	if err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: "internal error",
		}
	}

	// get a initiated transfer, get a lock, so not other workflow can pick it up
	transfer, err := db.GetTransaction(ctx, req.SourceAccount, req.Amount, db.TransferProgressInitiated, true, tx)
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

	req.WorkflowID = transfer.ID
	// signal the auth workflow to settle transaction
	err = s.temporalClient.SignalWorkflow(ctx, req.WorkflowID.String(), "", fmt.Sprintf("presentment-%s", req.WorkflowID.String()), &PresentmentSignal{ID: req.WorkflowID.String()})
	if err != nil {
		return err
	}

	// update the transfer progress to in progress so that the another workflow doesn't pick it up
	err = db.UpdateTransferProgress(transfer.ID, db.TransferProgressInProcess, tx)
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
