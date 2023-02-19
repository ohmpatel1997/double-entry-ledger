package ledger

import (
	"math/rand"

	tb "github.com/tigerbeetledb/tigerbeetle-go"
	tb_types "github.com/tigerbeetledb/tigerbeetle-go/pkg/types"
)

type ledgerService struct {
	TB tb.Client
}

func NewLedgerService(tb tb.Client) *ledgerService {
	return &ledgerService{
		TB: tb,
	}
}

func (l *ledgerService) CreateAccount(id uint64) (*tb_types.AccountEventResult, error) {
	resp, err := l.TB.CreateAccounts([]tb_types.Account{
		{
			ID:     id,
			Ledger: rand.Uint32(),
		},
	})

	if err != nil {
		return nil, err
	}

	return &resp[0], nil
}
