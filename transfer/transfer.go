package transfer

import (
	"context"
	"errors"
	"fmt"

	encore "encore.dev"
	"encore.dev/beta/errs"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"

	"encore.dev/types/uuid"

	"github.com/ohmpatel1997/pave-coding-challenge-simon/ledger"
	"github.com/ohmpatel1997/pave-coding-challenge-simon/transfer/db"
	"github.com/ohmpatel1997/pave-coding-challenge-simon/transfer/workflow"
	tb "github.com/tigerbeetledb/tigerbeetle-go"
)

// encore:service
type Service struct {
	client      client.Client
	worker      worker.Worker
	workflowSvc *workflow.Service
}

func initService() (*Service, error) {
	c, err := client.Dial(client.Options{})
	if err != nil {
		return nil, fmt.Errorf("create temporal client: %v", err)
	}

	tbClient, err := tb.NewClient(0, []string{"2000"}, 1)
	if err != nil {
		return nil, errs.Wrap(errors.New("error connecting to db"), err.Error())
	}
	ledgerSvc := ledger.NewLedgerService(tbClient)

	workflowSvc := workflow.NewService(ledgerSvc)

	w := worker.New(c, encore.Meta().Environment.Name+"-credit-card-transfer", worker.Options{})

	w.RegisterWorkflow(workflowSvc.Authorization)
	w.RegisterWorkflow(workflowSvc.Presentment)

	w.RegisterActivity(uuid.NewV4)
	w.RegisterActivity(ledgerSvc.FreezeAmount)
	w.RegisterActivity(ledgerSvc.SettleTransaction)
	w.RegisterActivity(ledgerSvc.CancelTransaction)
	w.RegisterActivity(db.InsertNewTransfer)
	w.RegisterActivity(db.UpdateTransferAsTimeout)
	w.RegisterActivity(db.UpdateTransferProgress)

	err = w.Start()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("start temporal worker: %v", err)
	}

	return &Service{client: c, worker: w, workflowSvc: workflowSvc}, nil
}

func (s *Service) Shutdown(force context.Context) {
	s.client.Close()
	s.worker.Stop()
	s.workflowSvc.LedgerSvc.TB.Close()
}

//encore:api private method=POST
func (s *Service) Transfer(ctx context.Context, req *Request) error {
	switch req.TxnType {
	case TransactionTypeCreditCardAuth:
		workflowID, err := uuid.NewV4()
		if err != nil {
			return fmt.Errorf("generate workflow id: %v", err)
		}

		options := client.StartWorkflowOptions{
			ID:        workflowID.String(),
			TaskQueue: encore.Meta().Environment.Name + "-credit-card-transfer",
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1, // try only once
			},
			//WorkflowExecutionTimeout: time.Minute * 5,
			//WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		}

		_, err = s.client.ExecuteWorkflow(ctx, options, s.workflowSvc.Authorization, &workflow.PaymentDetails{
			WorkflowID:    workflowID,
			SourceAccount: req.CustomerAccount,
			TargetAccount: 2, // bank's account, hardcoded for now
			Amount:        req.Amount,
		})
		if err != nil {
			return &errs.Error{
				Code:    errs.Internal,
				Message: errs.Wrap(err, "error executing workflow").Error(),
			}
		}
	case TransactionTypeCreditCardPresent:
		workflowID, err := uuid.NewV4()
		if err != nil {
			return fmt.Errorf("generate workflow id: %v", err)
		}

		options := client.StartWorkflowOptions{
			ID:        workflowID.String(),
			TaskQueue: encore.Meta().Environment.Name + "-credit-card-transfer",
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1, // try only once
			},
		}

		_, err = s.client.ExecuteWorkflow(ctx, options, s.workflowSvc.Presentment, ctx, &workflow.PaymentDetails{
			SourceAccount: req.CustomerAccount,
			TargetAccount: 2, // bank's account, hardcoded for now
			Amount:        req.Amount,
		})
		if err != nil {
			return &errs.Error{
				Code:    errs.Internal,
				Message: errs.Wrap(err, "error executing workflow").Error(),
			}
		}
	}

	return nil
}

type PresentmentRequest struct {
	Account uint64 `json:"account"`
	Amount  uint64 `json:"amount"`
}

// GetAuthTransferForPresentment returns the auth transfer for presentment
//
//encore:api private method=GET
func (s *Service) GetAuthTransferForPresentment(ctx context.Context, req *PresentmentRequest) (*db.TransferResponse, error) {
	return db.GetTransaction(ctx, req.Account, req.Amount, db.TransferProgressInitiated, false)
}
