package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"encore.dev/types/uuid"

	"github.com/ohmpatel1997/pave-coding-challenge-simon/ledger"
	"github.com/ohmpatel1997/pave-coding-challenge-simon/transfer/db"
)

type Service struct {
	LedgerSvc      *ledger.Service
	temporalClient client.Client
}

type PaymentDetails struct {
	WorkflowID    uuid.UUID
	SourceAccount uint64
	TargetAccount uint64
	Amount        uint64
}

func NewService(ledgerSvc *ledger.Service, temporalClient client.Client) *Service {
	return &Service{LedgerSvc: ledgerSvc, temporalClient: temporalClient}
}

type PresentmentSignal struct {
	ID string
}

func (s *Service) Authorization(ctx workflow.Context, paymentDetails *PaymentDetails) error {
	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval: time.Second,
		MaximumInterval: 5 * time.Second,
		MaximumAttempts: 5,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute, // maximum time of a single Activity execution
		RetryPolicy:         retrypolicy,
	}

	// workflow id would be the transaction id
	req := &ledger.TransferReq{
		ID:              paymentDetails.WorkflowID,
		DebitAccountID:  paymentDetails.SourceAccount,
		CreditAccountID: paymentDetails.TargetAccount,
		Amount:          paymentDetails.Amount,
	}

	err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), s.LedgerSvc.FreezeAmount, req).Get(ctx, nil)
	if err != nil {
		return err
	}

	// insert it into external db
	tnsfer := db.TransferReq{
		ID:              paymentDetails.WorkflowID,
		DebitAccountID:  paymentDetails.SourceAccount,
		CreditAccountID: paymentDetails.TargetAccount,
		Amount:          paymentDetails.Amount,
	}

	err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.InsertNewTransfer, tnsfer).Get(ctx, nil)
	if err != nil {
		// unfreeze the amount
		var cancelID uuid.UUID
		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), uuid.NewV4).Get(ctx, &cancelID)
		if err != nil {
			tnsfer.Progress = db.TransferProgressFailedOnLedgerCancellation
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.InsertNewTransferWithProgress, tnsfer).Get(ctx, nil)
			if err != nil {
				return err
			}
			return err
		}

		err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), s.LedgerSvc.CancelTransaction, req.ID, cancelID).Get(ctx, nil)
		if err != nil {
			tnsfer.Progress = db.TransferProgressFailedOnLedgerCancellation
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.InsertNewTransferWithProgress, tnsfer).Get(ctx, nil)
			if err != nil {
				return err
			}
			return err

		}
		return err
	}

	var signal PresentmentSignal
	var timedOut = false

	presentmentChan := workflow.GetSignalChannel(ctx, fmt.Sprintf("presentment-%s", tnsfer.ID.String()))

	futureCtx, futureCancel := workflow.WithCancel(ctx)
	defer futureCancel()
	timeoutFuture := workflow.NewTimer(futureCtx, 100*time.Second)
	selector := workflow.NewSelector(ctx)
	selector.AddReceive(presentmentChan, func(channel workflow.ReceiveChannel, more bool) {
		channel.Receive(ctx, &signal)
	})
	selector.AddFuture(timeoutFuture, func(future workflow.Future) {
		_ = future.Get(futureCtx, nil)
		timedOut = true
	})

	// wait for presentment signal or timeout
	selector.Select(ctx)

	switch {
	case timedOut, len(signal.ID) > 0 && signal.ID != req.ID.String(): // got the invalid signal transaction id. This case should never happen ideally
		// cancel the transaction

		var cancelID uuid.UUID
		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), uuid.NewV4).Get(ctx, &cancelID)
		if err != nil {
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressFailedOnLedgerTimeout, nil).Get(ctx, nil)
			if err != nil {
				return err
			}
			return err
		}

		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), s.LedgerSvc.CancelTransaction, req.ID, cancelID).Get(ctx, nil)
		if err != nil {
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressFailedOnLedgerTimeout, nil).Get(ctx, nil)
			if err != nil {
				return err
			}

			return err
		}

		// update the flag in external db
		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressSettled).Get(ctx, nil)
		if err != nil {
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressFailedOnExternalDB, nil).Get(ctx, nil)
			if err != nil {
				return err
			}

			return err
		}

	case len(signal.ID) > 0 && signal.ID == req.ID.String():
		// settle the transaction
		var settlementID uuid.UUID
		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), uuid.NewV4).Get(ctx, &settlementID)
		if err != nil {
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressFailedOnLedgerSettlement, nil).Get(ctx, nil)
			if err != nil {
				return err
			}

			return err
		}

		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), s.LedgerSvc.SettleTransaction, req.ID, settlementID).Get(ctx, nil)
		if err != nil {
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressFailedOnLedgerSettlement, nil).Get(ctx, nil)
			if err != nil {
				return err
			}
			return err
		}

		// update the flag in external db
		err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressSettled).Get(ctx, nil)
		if err != nil {
			// update the flag in external db
			err = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), db.UpdateTransferProgress, req.ID, db.TransferProgressFailedOnExternalDB, nil).Get(ctx, nil)
			if err != nil {
				return err
			}
			return err
		}
	}

	return nil
}

func (s *Service) Presentment(ctx workflow.Context, req *PaymentDetails) error {

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval: 100 * time.Millisecond,
		MaximumInterval: 2 * time.Second,
		MaximumAttempts: 5,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute, // maximum time of a single Activity execution
		RetryPolicy:         retrypolicy,
	}

	err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, options), s.SignalActivity, req).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
