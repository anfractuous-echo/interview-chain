package transaction

import (
	"bytes"
	"encoding/json"
)

type Signed struct {
	ID          string `json:"id"`
	PaymentID   string `json:"payment_id"`
	From        string `json:"from"`
	To          string `json:"to"`
	AmountUnits int64  `json:"amount_units"`
	Signature   string `json:"signature"`
}

type canonicalPayload struct {
	Version     int    `json:"version"`
	PaymentID   string `json:"payment_id"`
	From        string `json:"from"`
	To          string `json:"to"`
	AmountUnits int64  `json:"amount_units"`
}

func CanonicalPayload(tx Signed) ([]byte, error) {
	return json.Marshal(canonicalPayload{
		Version:     1,
		PaymentID:   tx.PaymentID,
		From:        tx.From,
		To:          tx.To,
		AmountUnits: tx.AmountUnits,
	})
}

func Equal(a, b Signed) bool {
	left, errLeft := json.Marshal(a)
	right, errRight := json.Marshal(b)
	return errLeft == nil && errRight == nil && bytes.Equal(left, right)
}
