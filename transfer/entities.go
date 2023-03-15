package transfer

type TransactionType string

const (
	TransactionTypeCreditCardAuth    TransactionType = "credit_card_authorization"
	TransactionTypeCreditCardPresent TransactionType = "credit_card_presentment"
)

type Request struct {
	//TransferID      *string
	CustomerAccount uint64
	TxnType         TransactionType
	Amount          uint64
}
