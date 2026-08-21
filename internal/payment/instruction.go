package payment

type Status string

const (
	StatusReceived  Status = "received"
	StatusSubmitted Status = "submitted"
)

type Instruction struct {
	ID          string `json:"payment_id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      string `json:"amount"`
	AmountUnits int64  `json:"-"`
	RawJSON     []byte `json:"-"`
	Status      Status `json:"-"`
}
